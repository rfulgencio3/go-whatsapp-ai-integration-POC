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
	conversations      ConversationRepository
	sourceMessages     SourceMessageRepository
	transcriptions     TranscriptionRepository
	interpretationRuns InterpretationRunRepository
	businessEvents     BusinessEventRepository
	assistantMessages  AssistantMessageRepository
}

func NewCaptureService(
	logger *observability.Logger,
	downstream chatbot.MessageProcessor,
	messageSender chatbot.MessageSender,
	chatHistory chatbot.ConversationRepository,
	messageArchive chatbot.MessageArchive,
	interpreter Interpreter,
	farmMemberships FarmMembershipRepository,
	conversations ConversationRepository,
	sourceMessages SourceMessageRepository,
	transcriptions TranscriptionRepository,
	interpretationRuns InterpretationRunRepository,
	businessEvents BusinessEventRepository,
	assistantMessages AssistantMessageRepository,
) *CaptureService {
	if logger == nil {
		logger = observability.NewLogger()
	}

	return &CaptureService{
		logger:             logger,
		downstream:         downstream,
		messageSender:      messageSender,
		chatHistory:        chatHistory,
		messageArchive:     messageArchive,
		interpreter:        interpreter,
		farmMemberships:    farmMemberships,
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

	membership, resolved, err := s.resolveMembership(ctx, message.PhoneNumber)
	if err != nil {
		return chatbot.ProcessResult{}, err
	}
	if resolved {
		handled, result, err := s.handleConfirmationMessage(ctx, membership, message)
		if err != nil {
			return chatbot.ProcessResult{}, err
		}
		if handled {
			return result, nil
		}
	}

	result, err := s.downstream.ProcessIncomingMessage(ctx, message)
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

func (s *CaptureService) resolveMembership(ctx context.Context, phoneNumber string) (domain.FarmMembership, bool, error) {
	if s.farmMemberships == nil {
		return domain.FarmMembership{}, false, nil
	}

	normalized := domain.NormalizePhoneNumber(phoneNumber)
	if normalized == "" {
		return domain.FarmMembership{}, false, nil
	}

	memberships, err := s.farmMemberships.FindActiveByPhoneNumber(ctx, normalized)
	if err != nil {
		return domain.FarmMembership{}, false, err
	}
	switch len(memberships) {
	case 0:
		return domain.FarmMembership{}, false, nil
	case 1:
		return memberships[0], true, nil
	default:
		s.logger.Info("agro context is ambiguous for inbound phone", map[string]any{
			"phone_number":      normalized,
			"matching_contexts": len(memberships),
		})
		return domain.FarmMembership{}, false, nil
	}
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
