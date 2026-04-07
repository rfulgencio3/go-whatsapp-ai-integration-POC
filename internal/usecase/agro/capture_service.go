package agro

import (
	"context"
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
		AnimalCode:          strings.TrimSpace(interpretation.AnimalCode),
		Status:              domain.EventStatusDraft,
		ConfirmedByUser:     false,
		CreatedAt:           occurredAt,
		UpdatedAt:           occurredAt,
	}

	if err := s.businessEvents.Create(ctx, &event); err != nil {
		return domain.BusinessEvent{}, false, err
	}
	if err := s.businessEvents.CreateAttributes(ctx, event.ID, interpretation.Attributes); err != nil {
		return domain.BusinessEvent{}, false, err
	}

	return event, interpretation.RequiresConfirmation, nil
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

func legacyBuildUnregisteredNumberReply() string {
	return "Seu numero ainda nao esta vinculado a uma fazenda. Peça o cadastro do seu telefone para continuar."
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
