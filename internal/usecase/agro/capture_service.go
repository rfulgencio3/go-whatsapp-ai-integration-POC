package agro

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type CaptureService struct {
	logger             *observability.Logger
	downstream         chatbot.MessageProcessor
	messageSender      chatbot.MessageSender
	chatHistory        chatbot.ConversationRepository
	messageArchive     chatbot.MessageArchive
	interpreter        Interpreter
	farmMemberships    FarmMembershipRepository
	farmRegistrations  FarmRegistrationRepository
	phoneContexts      PhoneContextStateRepository
	onboardingStates   OnboardingStateRepository
	onboardingMessages OnboardingMessageRepository
	conversations      ConversationRepository
	sourceMessages     SourceMessageRepository
	transcriptions     TranscriptionRepository
	interpretationRuns InterpretationRunRepository
	businessEvents     BusinessEventRepository
	assistantMessages  AssistantMessageRepository
}

type membershipResolution string

const (
	membershipResolutionUnavailable membershipResolution = "unavailable"
	membershipResolutionNotFound    membershipResolution = "not_found"
	membershipResolutionAmbiguous   membershipResolution = "ambiguous"
	membershipResolutionResolved    membershipResolution = "resolved"
	membershipResolutionSelected    membershipResolution = "selected"
)

func NewCaptureService(
	logger *observability.Logger,
	downstream chatbot.MessageProcessor,
	messageSender chatbot.MessageSender,
	chatHistory chatbot.ConversationRepository,
	messageArchive chatbot.MessageArchive,
	interpreter Interpreter,
	farmMemberships FarmMembershipRepository,
	farmRegistrations FarmRegistrationRepository,
	phoneContexts PhoneContextStateRepository,
	onboardingStates OnboardingStateRepository,
	conversations ConversationRepository,
	sourceMessages SourceMessageRepository,
	transcriptions TranscriptionRepository,
	interpretationRuns InterpretationRunRepository,
	businessEvents BusinessEventRepository,
	assistantMessages AssistantMessageRepository,
	onboardingMessages ...OnboardingMessageRepository,
) *CaptureService {
	if logger == nil {
		logger = observability.NewLogger()
	}

	var onboardingMessageRepository OnboardingMessageRepository
	if len(onboardingMessages) > 0 {
		onboardingMessageRepository = onboardingMessages[0]
	}

	return &CaptureService{
		logger:             logger,
		downstream:         downstream,
		messageSender:      messageSender,
		chatHistory:        chatHistory,
		messageArchive:     messageArchive,
		interpreter:        interpreter,
		farmMemberships:    farmMemberships,
		farmRegistrations:  farmRegistrations,
		phoneContexts:      phoneContexts,
		onboardingStates:   onboardingStates,
		onboardingMessages: onboardingMessageRepository,
		conversations:      conversations,
		sourceMessages:     sourceMessages,
		transcriptions:     transcriptions,
		interpretationRuns: interpretationRuns,
		businessEvents:     businessEvents,
		assistantMessages:  assistantMessages,
	}
}

func (s *CaptureService) ProcessIncomingMessage(ctx context.Context, message chat.IncomingMessage) (chatbot.ProcessResult, error) {
	if s.downstream == nil {
		return chatbot.ProcessResult{}, nil
	}
	handled, result, err := s.handleOnboarding(ctx, message)
	if err != nil {
		return chatbot.ProcessResult{}, err
	}
	if handled {
		return result, nil
	}
	handled, result, err = s.handleContextSwitchRequest(ctx, message)
	if err != nil {
		return chatbot.ProcessResult{}, err
	}
	if handled {
		return result, nil
	}

	membership, resolution, err := s.resolveMembership(ctx, message.PhoneNumber)
	if err != nil {
		return chatbot.ProcessResult{}, err
	}
	resolved := resolution == membershipResolutionResolved
	if resolved {
		handled, result, err := s.handleConfirmationMessage(ctx, membership, message)
		if err != nil {
			return chatbot.ProcessResult{}, err
		}
		if handled {
			return result, nil
		}
	}
	if !resolved {
		handled, result, err := s.handleUnresolvedMembership(ctx, resolution, message)
		if err != nil {
			return chatbot.ProcessResult{}, err
		}
		if handled {
			return result, nil
		}
	}

	result, err = s.downstream.ProcessIncomingMessage(ctx, message)
	if err != nil || result.Duplicate {
		return result, err
	}

	if err := s.captureProcessedInteraction(ctx, membership, resolved, result); err != nil {
		s.logger.Error("agro inbound capture failed", map[string]any{
			"phone_number": domain.NormalizePhoneNumber(result.PhoneNumber),
			"message_id":   strings.TrimSpace(result.IncomingMessage.MessageID),
			"provider":     strings.TrimSpace(result.IncomingMessage.Provider),
			"error":        err.Error(),
		})
	}

	return result, nil
}

func (s *CaptureService) handleOnboarding(ctx context.Context, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	if s.messageSender == nil || s.onboardingStates == nil {
		return false, chatbot.ProcessResult{}, nil
	}

	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)
	state, found, err := s.onboardingStates.GetByPhoneNumber(ctx, normalizedPhone)
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	replyText := ""
	traceStep := domain.OnboardingStep("")
	switch {
	case isOnboardingStartCommand(message.Text):
		replyText, traceStep, err = s.startOnboarding(ctx, normalizedPhone)
	case found && state.Step == domain.OnboardingStepAwaitingProducerName:
		replyText, traceStep, err = s.completeOnboardingProducerStep(ctx, state, message.Text)
	case found && state.Step == domain.OnboardingStepAwaitingFarmName:
		replyText, traceStep, err = s.completeOnboardingFarmStep(ctx, state, message.Text)
	default:
		return false, chatbot.ProcessResult{}, nil
	}
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	now := time.Now().UTC()
	if err := s.messageSender.SendTextMessage(ctx, normalizedPhone, replyText); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	userMessage := buildChatMessageFromIncoming(message, strings.TrimSpace(message.Text))
	assistantMessage := chat.Message{
		Role:      chat.AssistantRole,
		Text:      replyText,
		CreatedAt: now,
		Type:      chat.MessageTypeText,
		Provider:  providerOrDefault(message.Provider),
	}
	if err := s.persistLegacyConversation(ctx, normalizedPhone, userMessage, assistantMessage); err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if err := s.persistOnboardingInteraction(ctx, normalizedPhone, traceStep, message, assistantMessage, now); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	return true, chatbot.ProcessResult{
		PhoneNumber:      normalizedPhone,
		IncomingMessage:  message,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
	}, nil
}

func (s *CaptureService) startOnboarding(ctx context.Context, phoneNumber string) (string, domain.OnboardingStep, error) {
	if s.farmMemberships != nil {
		memberships, err := s.farmMemberships.FindActiveByPhoneNumber(ctx, phoneNumber)
		if err != nil {
			return "", "", err
		}
		if len(memberships) > 0 {
			return buildAlreadyRegisteredReply(), "", nil
		}
	}

	state := domain.OnboardingState{
		PhoneNumber: phoneNumber,
		Step:        domain.OnboardingStepAwaitingProducerName,
		UpdatedAt:   time.Now().UTC(),
	}
	if err := s.onboardingStates.Upsert(ctx, &state); err != nil {
		return "", "", err
	}

	return "Vamos fazer seu cadastro inicial. Qual o nome do produtor ou responsavel?", domain.OnboardingStepAwaitingProducerName, nil
}

func (s *CaptureService) completeOnboardingProducerStep(ctx context.Context, state domain.OnboardingState, text string) (string, domain.OnboardingStep, error) {
	producerName := strings.TrimSpace(text)
	if producerName == "" {
		return "Envie o nome do produtor ou responsavel para continuar o cadastro.", domain.OnboardingStepAwaitingProducerName, nil
	}

	state.Step = domain.OnboardingStepAwaitingFarmName
	state.ProducerName = producerName
	state.UpdatedAt = time.Now().UTC()
	if err := s.onboardingStates.Upsert(ctx, &state); err != nil {
		return "", "", err
	}

	return "Agora envie o nome da fazenda ou negocio.", domain.OnboardingStepAwaitingFarmName, nil
}

func (s *CaptureService) completeOnboardingFarmStep(ctx context.Context, state domain.OnboardingState, text string) (string, domain.OnboardingStep, error) {
	farmName := strings.TrimSpace(text)
	if farmName == "" {
		return "Envie o nome da fazenda ou negocio para concluir o cadastro.", domain.OnboardingStepAwaitingFarmName, nil
	}
	if s.farmRegistrations == nil {
		return "", "", nil
	}

	membership, err := s.farmRegistrations.CreateInitialRegistration(ctx, state.PhoneNumber, state.ProducerName, farmName)
	if err != nil {
		return "", "", err
	}
	if s.onboardingStates != nil {
		if err := s.onboardingStates.DeleteByPhoneNumber(ctx, state.PhoneNumber); err != nil {
			return "", "", err
		}
	}

	return fmt.Sprintf("Cadastro concluido. Seu numero foi vinculado a %s. Agora voce ja pode enviar registros.", fallbackFarmName(domain.PhoneContextOption{FarmName: membership.FarmName}, 1)), "", nil
}

func (s *CaptureService) handleContextSwitchRequest(ctx context.Context, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	if s.messageSender == nil || s.farmMemberships == nil || s.phoneContexts == nil {
		return false, chatbot.ProcessResult{}, nil
	}
	if !isContextSwitchCommand(message.Text) {
		return false, chatbot.ProcessResult{}, nil
	}

	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)
	memberships, err := s.farmMemberships.FindActiveByPhoneNumber(ctx, normalizedPhone)
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	replyText := ""
	switch len(memberships) {
	case 0:
		replyText = buildUnregisteredNumberReply()
	case 1:
		replyText = buildSingleContextReply(memberships[0].FarmName)
	default:
		options := make([]domain.PhoneContextOption, 0, len(memberships))
		for _, membership := range memberships {
			options = append(options, domain.PhoneContextOption{
				FarmID:   membership.FarmID,
				FarmName: membership.FarmName,
			})
		}
		if err := s.phoneContexts.Upsert(ctx, &domain.PhoneContextState{
			PhoneNumber:    normalizedPhone,
			PendingOptions: options,
			UpdatedAt:      time.Now().UTC(),
		}); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		replyText = buildAmbiguousContextSelectionReply(options)
	}

	now := time.Now().UTC()
	if err := s.messageSender.SendTextMessage(ctx, normalizedPhone, replyText); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	userMessage := buildChatMessageFromIncoming(message, strings.TrimSpace(message.Text))
	assistantMessage := chat.Message{
		Role:      chat.AssistantRole,
		Text:      replyText,
		CreatedAt: now,
		Type:      chat.MessageTypeText,
		Provider:  providerOrDefault(message.Provider),
	}
	if err := s.persistLegacyConversation(ctx, normalizedPhone, userMessage, assistantMessage); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	return true, chatbot.ProcessResult{
		PhoneNumber:      normalizedPhone,
		IncomingMessage:  message,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
	}, nil
}

func (s *CaptureService) captureProcessedInteraction(ctx context.Context, membership domain.FarmMembership, resolved bool, result chatbot.ProcessResult) error {
	if !resolved || s.conversations == nil || s.sourceMessages == nil {
		return nil
	}

	phoneNumber := domain.NormalizePhoneNumber(result.PhoneNumber)
	if phoneNumber == "" {
		return nil
	}

	receivedAt := time.Now().UTC()
	conversation, err := s.conversations.GetOrCreateOpen(ctx, membership.FarmID, "whatsapp", phoneNumber, receivedAt)
	if err != nil {
		return err
	}

	sourceMessage := domain.SourceMessage{
		ID:                uuid.NewString(),
		ConversationID:    conversation.ID,
		Provider:          providerOrDefault(result.IncomingMessage.Provider),
		ProviderMessageID: strings.TrimSpace(result.IncomingMessage.MessageID),
		SenderPhoneNumber: phoneNumber,
		MessageType:       toDomainMessageType(result.IncomingMessage.Type),
		RawText:           strings.TrimSpace(result.IncomingMessage.Text),
		ReceivedAt:        receivedAt,
		CreatedAt:         receivedAt,
	}
	if len(result.IncomingMessage.MediaAttachments) > 0 {
		sourceMessage.MediaURL = strings.TrimSpace(result.IncomingMessage.MediaAttachments[0].URL)
		sourceMessage.MediaContentType = strings.TrimSpace(result.IncomingMessage.MediaAttachments[0].ContentType)
		sourceMessage.MediaFilename = strings.TrimSpace(result.IncomingMessage.MediaAttachments[0].Filename)
	}

	if err := s.sourceMessages.Create(ctx, &sourceMessage); err != nil {
		return err
	}
	transcriptionID, err := s.persistTranscription(ctx, sourceMessage.ID, result.IncomingMessage, receivedAt)
	if err != nil {
		return err
	}
	event, requiresConfirmation, err := s.persistInterpretation(ctx, membership.FarmID, sourceMessage, transcriptionID, receivedAt)
	if err != nil {
		return err
	}
	replyType := domain.ReplyTypeText
	if result.AssistantReplyKind == chatbot.ReplyKindConfirmation {
		replyType = domain.ReplyTypeConfirmation
	}
	if err := s.persistAssistantMessage(ctx, conversation.ID, sourceMessage.ID, result.AssistantMessage, replyType, receivedAt); err != nil {
		return err
	}
	if requiresConfirmation {
		if err := s.conversations.SetPendingConfirmationEvent(ctx, conversation.ID, event.ID); err != nil {
			return err
		}
	}
	if strings.TrimSpace(conversation.PendingCorrectionEventID) != "" {
		if err := s.businessEvents.CreateCorrectionLink(ctx, event.ID, conversation.PendingCorrectionEventID); err != nil {
			return err
		}
		if err := s.conversations.SetPendingCorrectionEvent(ctx, conversation.ID, ""); err != nil {
			return err
		}
	}
	if requiresConfirmation && result.AssistantReplyKind != chatbot.ReplyKindConfirmation {
		if err := s.sendDraftConfirmationPrompt(ctx, phoneNumber, conversation.ID, sourceMessage.ID, event, receivedAt); err != nil {
			return err
		}
	}

	return nil
}

func (s *CaptureService) sendDraftConfirmationPrompt(ctx context.Context, phoneNumber, conversationID, sourceMessageID string, event domain.BusinessEvent, createdAt time.Time) error {
	if s.messageSender == nil {
		return nil
	}
	assistantMessage := chat.Message{
		Role:      chat.AssistantRole,
		Text:      buildDraftConfirmationPrompt(event),
		CreatedAt: createdAt,
		Type:      chat.MessageTypeText,
		Provider:  "whatsapp",
	}
	if err := s.messageSender.SendTextMessage(ctx, phoneNumber, assistantMessage.Text); err != nil {
		return err
	}
	if s.chatHistory != nil && s.messageArchive != nil {
		if err := s.chatHistory.AppendMessage(ctx, phoneNumber, assistantMessage); err != nil {
			return err
		}
		if err := s.messageArchive.RecordMessage(ctx, phoneNumber, assistantMessage); err != nil {
			return err
		}
	}
	if err := s.persistAssistantMessage(ctx, conversationID, sourceMessageID, assistantMessage, domain.ReplyTypeConfirmation, createdAt); err != nil {
		return err
	}

	return nil
}

func providerOrDefault(provider string) string {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "whatsapp"
	}

	return provider
}

func toDomainMessageType(messageType chat.MessageType) domain.MessageType {
	switch messageType {
	case chat.MessageTypeText:
		return domain.MessageTypeText
	case chat.MessageTypeAudio:
		return domain.MessageTypeAudio
	case chat.MessageTypeImage:
		return domain.MessageTypeImage
	case chat.MessageTypeDocument:
		return domain.MessageTypeDocument
	default:
		return domain.MessageTypeUnsupported
	}
}

func (s *CaptureService) persistTranscription(ctx context.Context, sourceMessageID string, incomingMessage chat.IncomingMessage, createdAt time.Time) (string, error) {
	if s.transcriptions == nil || strings.TrimSpace(incomingMessage.TranscriptionID) == "" {
		return "", nil
	}

	transcription := domain.Transcription{
		ID:              uuid.NewString(),
		SourceMessageID: sourceMessageID,
		Provider:        "transcription-api",
		ProviderRef:     strings.TrimSpace(incomingMessage.TranscriptionID),
		TranscriptText:  strings.TrimSpace(incomingMessage.Text),
		Language:        strings.TrimSpace(incomingMessage.TranscriptionLanguage),
		DurationSeconds: incomingMessage.AudioDurationSeconds,
		CreatedAt:       createdAt,
	}

	if err := s.transcriptions.Create(ctx, &transcription); err != nil {
		return "", err
	}

	return transcription.ID, nil
}

func (s *CaptureService) persistAssistantMessage(ctx context.Context, conversationID, sourceMessageID string, assistantMessage chat.Message, replyType domain.ReplyType, createdAt time.Time) error {
	if s.assistantMessages == nil || strings.TrimSpace(assistantMessage.Text) == "" {
		return nil
	}
	if replyType == "" {
		replyType = domain.ReplyTypeText
	}

	message := domain.AssistantMessage{
		ID:                uuid.NewString(),
		ConversationID:    conversationID,
		SourceMessageID:   sourceMessageID,
		Provider:          providerOrDefault(assistantMessage.Provider),
		ProviderMessageID: strings.TrimSpace(assistantMessage.ProviderMessageID),
		ReplyType:         replyType,
		Body:              strings.TrimSpace(assistantMessage.Text),
		CreatedAt:         createdAt,
	}

	return s.assistantMessages.Create(ctx, &message)
}

func (s *CaptureService) persistInterpretation(ctx context.Context, farmID string, sourceMessage domain.SourceMessage, transcriptionID string, occurredAt time.Time) (domain.BusinessEvent, bool, error) {
	if s.interpreter == nil || s.interpretationRuns == nil {
		return domain.BusinessEvent{}, false, nil
	}

	interpretation, err := s.interpreter.Interpret(ctx, InterpretationInput{
		MessageType: sourceMessage.MessageType,
		Text:        sourceMessage.RawText,
		OccurredAt:  occurredAt,
	})
	if err != nil {
		return domain.BusinessEvent{}, false, err
	}
	if strings.TrimSpace(interpretation.NormalizedIntent) == "" {
		return domain.BusinessEvent{}, false, nil
	}

	run := domain.InterpretationRun{
		ID:                   uuid.NewString(),
		SourceMessageID:      sourceMessage.ID,
		TranscriptionID:      strings.TrimSpace(transcriptionID),
		ModelProvider:        interpreterProvider,
		ModelName:            interpreterModel,
		PromptVersion:        promptVersion,
		NormalizedIntent:     interpretation.NormalizedIntent,
		Confidence:           interpretation.Confidence,
		RequiresConfirmation: interpretation.RequiresConfirmation,
		RawOutputJSON:        interpretation.RawOutputJSON,
		CreatedAt:            occurredAt,
	}
	if err := s.interpretationRuns.Create(ctx, &run); err != nil {
		return domain.BusinessEvent{}, false, err
	}
	if s.businessEvents == nil {
		return domain.BusinessEvent{}, false, nil
	}

	event := domain.BusinessEvent{
		ID:                  uuid.NewString(),
		FarmID:              farmID,
		SourceMessageID:     sourceMessage.ID,
		InterpretationRunID: run.ID,
		Category:            interpretation.Category,
		Subcategory:         interpretation.Subcategory,
		OccurredAt:          interpretation.OccurredAt,
		Description:         interpretation.Description,
		Amount:              interpretation.Amount,
		Currency:            interpretation.Currency,
		Quantity:            interpretation.Quantity,
		Unit:                interpretation.Unit,
		Status:              domain.EventStatusDraft,
		ConfirmedByUser:     false,
		CreatedAt:           occurredAt,
		UpdatedAt:           occurredAt,
	}

	if err := s.businessEvents.Create(ctx, &event); err != nil {
		return domain.BusinessEvent{}, false, err
	}

	return event, interpretation.RequiresConfirmation, nil
}

func (s *CaptureService) resolveMembership(ctx context.Context, phoneNumber string) (domain.FarmMembership, membershipResolution, error) {
	if s.farmMemberships == nil {
		return domain.FarmMembership{}, membershipResolutionUnavailable, nil
	}

	normalized := domain.NormalizePhoneNumber(phoneNumber)
	if normalized == "" {
		return domain.FarmMembership{}, membershipResolutionUnavailable, nil
	}

	memberships, err := s.farmMemberships.FindActiveByPhoneNumber(ctx, normalized)
	if err != nil {
		return domain.FarmMembership{}, membershipResolutionUnavailable, err
	}
	if len(memberships) > 1 && s.phoneContexts != nil {
		state, found, err := s.phoneContexts.GetByPhoneNumber(ctx, normalized)
		if err != nil {
			return domain.FarmMembership{}, membershipResolutionUnavailable, err
		}
		if found && strings.TrimSpace(state.ActiveFarmID) != "" {
			for _, membership := range memberships {
				if membership.FarmID == state.ActiveFarmID {
					return membership, membershipResolutionResolved, nil
				}
			}
		}
	}
	switch len(memberships) {
	case 0:
		return domain.FarmMembership{}, membershipResolutionNotFound, nil
	case 1:
		return memberships[0], membershipResolutionResolved, nil
	default:
		s.logger.Info("agro context is ambiguous for inbound phone", map[string]any{
			"phone_number":      normalized,
			"matching_contexts": len(memberships),
		})
		return domain.FarmMembership{}, membershipResolutionAmbiguous, nil
	}
}

func (s *CaptureService) handleUnresolvedMembership(ctx context.Context, resolution membershipResolution, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	if s.messageSender == nil {
		return false, chatbot.ProcessResult{}, nil
	}

	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)
	replyText := ""
	switch resolution {
	case membershipResolutionNotFound:
		replyText = buildUnregisteredNumberReply()
	case membershipResolutionAmbiguous:
		handled, responseText, err := s.handleAmbiguousMembershipSelection(ctx, normalizedPhone, message.Text)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		if !handled {
			return false, chatbot.ProcessResult{}, nil
		}
		replyText = responseText
	default:
		return false, chatbot.ProcessResult{}, nil
	}

	now := time.Now().UTC()
	if err := s.messageSender.SendTextMessage(ctx, normalizedPhone, replyText); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	userMessage := buildChatMessageFromIncoming(message, strings.TrimSpace(message.Text))
	assistantMessage := chat.Message{
		Role:      chat.AssistantRole,
		Text:      replyText,
		CreatedAt: now,
		Type:      chat.MessageTypeText,
		Provider:  providerOrDefault(message.Provider),
	}
	if err := s.persistLegacyConversation(ctx, normalizedPhone, userMessage, assistantMessage); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	return true, chatbot.ProcessResult{
		PhoneNumber:      normalizedPhone,
		IncomingMessage:  message,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
	}, nil
}

func (s *CaptureService) handleAmbiguousMembershipSelection(ctx context.Context, phoneNumber, text string) (bool, string, error) {
	if s.farmMemberships == nil || s.phoneContexts == nil {
		return true, buildAmbiguousContextReply(), nil
	}

	state, found, err := s.phoneContexts.GetByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return false, "", err
	}
	if found && len(state.PendingOptions) > 0 {
		selection := parseContextSelection(text)
		if selection >= 1 && selection <= len(state.PendingOptions) {
			option := state.PendingOptions[selection-1]
			state.ActiveFarmID = option.FarmID
			state.PendingOptions = nil
			state.UpdatedAt = time.Now().UTC()
			if err := s.phoneContexts.Upsert(ctx, &state); err != nil {
				return false, "", err
			}

			return true, buildSelectedContextReply(option.FarmName), nil
		}
	}

	memberships, err := s.farmMemberships.FindActiveByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return false, "", err
	}
	if len(memberships) == 0 {
		return true, buildUnregisteredNumberReply(), nil
	}

	options := make([]domain.PhoneContextOption, 0, len(memberships))
	for _, membership := range memberships {
		options = append(options, domain.PhoneContextOption{
			FarmID:   membership.FarmID,
			FarmName: membership.FarmName,
		})
	}
	state = domain.PhoneContextState{
		PhoneNumber:    phoneNumber,
		PendingOptions: options,
		UpdatedAt:      time.Now().UTC(),
	}
	if err := s.phoneContexts.Upsert(ctx, &state); err != nil {
		return false, "", err
	}

	return true, buildAmbiguousContextSelectionReply(options), nil
}

func (s *CaptureService) handleConfirmationMessage(ctx context.Context, membership domain.FarmMembership, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	decision := classifyConfirmationDecision(message.Text)
	if decision == "" || s.businessEvents == nil || s.messageSender == nil || s.conversations == nil {
		return false, chatbot.ProcessResult{}, nil
	}

	now := time.Now().UTC()
	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)
	conversation, err := s.conversations.GetOrCreateOpen(ctx, membership.FarmID, "whatsapp", normalizedPhone, now)
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if strings.TrimSpace(conversation.PendingConfirmationEventID) == "" {
		return false, chatbot.ProcessResult{}, nil
	}

	event, found, err := s.businessEvents.FindByID(ctx, conversation.PendingConfirmationEventID)
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if !found || event.Status != domain.EventStatusDraft {
		return false, chatbot.ProcessResult{}, nil
	}
	status := domain.EventStatusRejected
	confirmedByUser := false
	replyText := buildRejectedReply()
	if decision == confirmationAccepted {
		status = domain.EventStatusConfirmed
		confirmedByUser = true
		replyText = buildConfirmedReply(event)
	}
	var confirmedAt *time.Time
	if confirmedByUser {
		confirmedAt = &now
	}

	if err := s.businessEvents.UpdateStatus(ctx, event.ID, status, confirmedByUser, confirmedAt); err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if err := s.conversations.SetPendingConfirmationEvent(ctx, conversation.ID, ""); err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if decision == confirmationRejected {
		if err := s.conversations.SetPendingCorrectionEvent(ctx, conversation.ID, event.ID); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
	} else {
		if err := s.conversations.SetPendingCorrectionEvent(ctx, conversation.ID, ""); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
	}

	savedConversation, sourceMessage, err := s.persistConfirmationInbound(ctx, membership, message, now)
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	normalizedPhone = domain.NormalizePhoneNumber(message.PhoneNumber)
	if err := s.messageSender.SendTextMessage(ctx, normalizedPhone, replyText); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	userMessage := buildChatMessageFromIncoming(message, strings.TrimSpace(message.Text))
	assistantMessage := chat.Message{
		Role:      chat.AssistantRole,
		Text:      replyText,
		CreatedAt: now,
		Type:      chat.MessageTypeText,
		Provider:  providerOrDefault(message.Provider),
	}
	if err := s.persistLegacyConversation(ctx, normalizedPhone, userMessage, assistantMessage); err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if err := s.persistAssistantMessage(ctx, savedConversation.ID, sourceMessage.ID, assistantMessage, domain.ReplyTypeConfirmation, now); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	return true, chatbot.ProcessResult{
		PhoneNumber:      normalizedPhone,
		IncomingMessage:  message,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
	}, nil
}

func (s *CaptureService) persistConfirmationInbound(ctx context.Context, membership domain.FarmMembership, message chat.IncomingMessage, receivedAt time.Time) (domain.Conversation, domain.SourceMessage, error) {
	if s.conversations == nil || s.sourceMessages == nil {
		return domain.Conversation{}, domain.SourceMessage{}, nil
	}

	phoneNumber := domain.NormalizePhoneNumber(message.PhoneNumber)
	conversation, err := s.conversations.GetOrCreateOpen(ctx, membership.FarmID, "whatsapp", phoneNumber, receivedAt)
	if err != nil {
		return domain.Conversation{}, domain.SourceMessage{}, err
	}

	sourceMessage := domain.SourceMessage{
		ID:                uuid.NewString(),
		ConversationID:    conversation.ID,
		Provider:          providerOrDefault(message.Provider),
		ProviderMessageID: strings.TrimSpace(message.MessageID),
		SenderPhoneNumber: phoneNumber,
		MessageType:       toDomainMessageType(message.Type),
		RawText:           strings.TrimSpace(message.Text),
		ReceivedAt:        receivedAt,
		CreatedAt:         receivedAt,
	}
	if err := s.sourceMessages.Create(ctx, &sourceMessage); err != nil {
		return domain.Conversation{}, domain.SourceMessage{}, err
	}

	return conversation, sourceMessage, nil
}

func (s *CaptureService) persistLegacyConversation(ctx context.Context, phoneNumber string, userMessage, assistantMessage chat.Message) error {
	if s.chatHistory == nil || s.messageArchive == nil {
		return nil
	}
	if err := s.chatHistory.AppendMessage(ctx, phoneNumber, userMessage); err != nil {
		return err
	}
	if err := s.messageArchive.RecordMessage(ctx, phoneNumber, userMessage); err != nil {
		return err
	}
	if err := s.chatHistory.AppendMessage(ctx, phoneNumber, assistantMessage); err != nil {
		return err
	}
	if err := s.messageArchive.RecordMessage(ctx, phoneNumber, assistantMessage); err != nil {
		return err
	}

	return nil
}

func (s *CaptureService) persistOnboardingInteraction(ctx context.Context, phoneNumber string, step domain.OnboardingStep, incomingMessage chat.IncomingMessage, assistantMessage chat.Message, createdAt time.Time) error {
	if s.onboardingMessages == nil {
		return nil
	}

	userBody := strings.TrimSpace(incomingMessage.Text)
	if userBody != "" {
		userMessage := domain.OnboardingMessage{
			ID:                uuid.NewString(),
			PhoneNumber:       phoneNumber,
			Step:              step,
			Direction:         domain.OnboardingMessageDirectionInbound,
			Provider:          providerOrDefault(incomingMessage.Provider),
			ProviderMessageID: strings.TrimSpace(incomingMessage.MessageID),
			MessageType:       toDomainMessageType(incomingMessage.Type),
			Body:              userBody,
			CreatedAt:         createdAt,
		}
		if err := s.onboardingMessages.Create(ctx, &userMessage); err != nil {
			return err
		}
	}

	replyBody := strings.TrimSpace(assistantMessage.Text)
	if replyBody == "" {
		return nil
	}

	reply := domain.OnboardingMessage{
		ID:          uuid.NewString(),
		PhoneNumber: phoneNumber,
		Step:        step,
		Direction:   domain.OnboardingMessageDirectionOutbound,
		Provider:    providerOrDefault(assistantMessage.Provider),
		MessageType: domain.MessageTypeText,
		Body:        replyBody,
		CreatedAt:   createdAt,
	}
	if err := s.onboardingMessages.Create(ctx, &reply); err != nil {
		return err
	}

	return nil
}

type confirmationDecision string

const (
	confirmationAccepted confirmationDecision = "accepted"
	confirmationRejected confirmationDecision = "rejected"
)

func classifyConfirmationDecision(text string) confirmationDecision {
	normalized := normalizeText(text)
	switch normalized {
	case "sim", "s", "ok", "confirmar", "confirmado", "pode confirmar", "isso":
		return confirmationAccepted
	case "nao", "não", "n", "cancelar", "corrigir", "errado":
		return confirmationRejected
	default:
		return ""
	}
}

func buildConfirmedReply(event domain.BusinessEvent) string {
	switch {
	case event.Category == "finance" && event.Subcategory == "input_purchase" && event.Amount != nil && event.Quantity != nil && strings.TrimSpace(event.Unit) != "":
		return fmt.Sprintf("Registro confirmado: compra de insumos de R$ %.2f, %.3g %s.", *event.Amount, *event.Quantity, event.Unit)
	case event.Category == "finance" && event.Subcategory == "expense" && event.Amount != nil:
		return fmt.Sprintf("Registro confirmado: despesa de R$ %.2f.", *event.Amount)
	case event.Category == "finance" && event.Subcategory == "revenue" && event.Amount != nil:
		return fmt.Sprintf("Registro confirmado: receita de R$ %.2f.", *event.Amount)
	case event.Category == "reproduction" && event.Subcategory == "insemination":
		return "Registro confirmado: evento de inseminacao salvo."
	default:
		return "Registro confirmado com sucesso."
	}
}

func buildRejectedReply() string {
	return "Entendi. Nao vou considerar esse registro. Envie a correcao em uma unica mensagem."
}

func buildUnregisteredNumberReply() string {
	return "Seu numero ainda nao esta vinculado a uma fazenda. Peça o cadastro do seu telefone para continuar."
}

func buildAmbiguousContextReply() string {
	return "Seu numero esta vinculado a mais de uma fazenda. Ajuste o cadastro antes de continuar."
}

func buildSingleContextReply(farmName string) string {
	return fmt.Sprintf("Seu numero ja esta vinculado a %s.", fallbackFarmName(domain.PhoneContextOption{FarmName: farmName}, 1))
}

func buildAmbiguousContextSelectionReply(options []domain.PhoneContextOption) string {
	var builder strings.Builder
	builder.WriteString("Seu numero esta vinculado a mais de uma fazenda. Responda com o numero:\n")
	for index, option := range options {
		builder.WriteString(fmt.Sprintf("%d. %s", index+1, fallbackFarmName(option, index+1)))
		if index < len(options)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func buildSelectedContextReply(farmName string) string {
	return fmt.Sprintf("Contexto definido para %s. Envie a informacao novamente.", fallbackFarmName(domain.PhoneContextOption{FarmName: farmName}, 1))
}

func parseContextSelection(text string) int {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return 0
	}
	if len(normalized) != 1 || normalized[0] < '1' || normalized[0] > '9' {
		return 0
	}

	return int(normalized[0] - '0')
}

func isContextSwitchCommand(text string) bool {
	switch normalizeText(text) {
	case "trocar fazenda", "mudar fazenda", "alternar fazenda", "selecionar fazenda", "trocar contexto", "mudar contexto":
		return true
	default:
		return false
	}
}

func isOnboardingStartCommand(text string) bool {
	switch normalizeText(text) {
	case "cadastrar", "cadastro", "quero cadastrar", "iniciar cadastro", "me cadastrar":
		return true
	default:
		return false
	}
}

func buildAlreadyRegisteredReply() string {
	return "Seu numero ja esta cadastrado. Pode enviar seus registros normalmente."
}

func fallbackFarmName(option domain.PhoneContextOption, position int) string {
	if strings.TrimSpace(option.FarmName) != "" {
		return strings.TrimSpace(option.FarmName)
	}

	return fmt.Sprintf("Fazenda %d", position)
}

func buildDraftConfirmationPrompt(event domain.BusinessEvent) string {
	return buildDraftConfirmationPromptFromInterpretation(InterpretationResult{
		Category:    event.Category,
		Subcategory: event.Subcategory,
		Amount:      event.Amount,
		Quantity:    event.Quantity,
		Unit:        event.Unit,
	})
}

func buildDraftConfirmationPromptFromInterpretation(result InterpretationResult) string {
	switch {
	case result.Category == "finance" && result.Subcategory == "input_purchase" && result.Amount != nil && result.Quantity != nil && strings.TrimSpace(result.Unit) != "":
		return fmt.Sprintf("Registrei compra de insumos de R$ %.2f, %.3g %s. Responda SIM para confirmar ou NAO para corrigir.", *result.Amount, *result.Quantity, result.Unit)
	case result.Category == "finance" && result.Subcategory == "expense" && result.Amount != nil:
		return fmt.Sprintf("Registrei despesa de R$ %.2f. Responda SIM para confirmar ou NAO para corrigir.", *result.Amount)
	case result.Category == "finance" && result.Subcategory == "revenue" && result.Amount != nil:
		return fmt.Sprintf("Registrei receita de R$ %.2f. Responda SIM para confirmar ou NAO para corrigir.", *result.Amount)
	case result.Category == "reproduction" && result.Subcategory == "insemination":
		return "Registrei um evento de inseminacao. Responda SIM para confirmar ou NAO para corrigir."
	default:
		return "Registrei essa informacao. Responda SIM para confirmar ou NAO para corrigir."
	}
}

func buildChatMessageFromIncoming(incomingMessage chat.IncomingMessage, text string) chat.Message {
	message := chat.Message{
		Role:                  chat.UserRole,
		Text:                  text,
		CreatedAt:             time.Now().UTC(),
		Type:                  incomingMessage.Type,
		Provider:              strings.TrimSpace(incomingMessage.Provider),
		ProviderMessageID:     strings.TrimSpace(incomingMessage.MessageID),
		TranscriptionID:       strings.TrimSpace(incomingMessage.TranscriptionID),
		TranscriptionLanguage: strings.TrimSpace(incomingMessage.TranscriptionLanguage),
		AudioDurationSeconds:  incomingMessage.AudioDurationSeconds,
	}
	if message.Type == "" {
		message.Type = chat.MessageTypeText
	}
	if len(incomingMessage.MediaAttachments) > 0 {
		message.MediaURL = strings.TrimSpace(incomingMessage.MediaAttachments[0].URL)
		message.MediaContentType = strings.TrimSpace(incomingMessage.MediaAttachments[0].ContentType)
		message.MediaFilename = strings.TrimSpace(incomingMessage.MediaAttachments[0].Filename)
	}
	return message
}
