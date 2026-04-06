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

	result, err := s.downstream.ProcessIncomingMessage(ctx, message)
	if err != nil || result.Duplicate {
		return result, err
	}

	if err := s.captureProcessedInteraction(ctx, result); err != nil {
		s.logger.Error("agro inbound capture failed", map[string]any{
			"phone_number": domain.NormalizePhoneNumber(result.PhoneNumber),
			"message_id":   strings.TrimSpace(result.IncomingMessage.MessageID),
			"provider":     strings.TrimSpace(result.IncomingMessage.Provider),
			"error":        err.Error(),
		})
	}

	return result, nil
}

func (s *CaptureService) captureProcessedInteraction(ctx context.Context, result chatbot.ProcessResult) error {
	if s.farmMemberships == nil || s.conversations == nil || s.sourceMessages == nil {
		return nil
	}

	phoneNumber := domain.NormalizePhoneNumber(result.PhoneNumber)
	if phoneNumber == "" {
		return nil
	}

	memberships, err := s.farmMemberships.FindActiveByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return err
	}

	switch len(memberships) {
	case 0:
		s.logger.Info("agro context not found for inbound phone", map[string]any{
			"phone_number": phoneNumber,
			"provider":     strings.TrimSpace(result.IncomingMessage.Provider),
		})
		return nil
	case 1:
	default:
		s.logger.Info("agro context is ambiguous for inbound phone", map[string]any{
			"phone_number":      phoneNumber,
			"provider":          strings.TrimSpace(result.IncomingMessage.Provider),
			"matching_contexts": len(memberships),
		})
		return nil
	}

	receivedAt := time.Now().UTC()
	conversation, err := s.conversations.GetOrCreateOpen(ctx, memberships[0].FarmID, "whatsapp", phoneNumber, receivedAt)
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
	if err := s.persistInterpretation(ctx, memberships[0].FarmID, sourceMessage, transcriptionID, receivedAt); err != nil {
		return err
	}
	if err := s.persistAssistantMessage(ctx, conversation.ID, sourceMessage.ID, result.AssistantMessage, receivedAt); err != nil {
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

func (s *CaptureService) persistAssistantMessage(ctx context.Context, conversationID, sourceMessageID string, assistantMessage chat.Message, createdAt time.Time) error {
	if s.assistantMessages == nil || strings.TrimSpace(assistantMessage.Text) == "" {
		return nil
	}

	message := domain.AssistantMessage{
		ID:                uuid.NewString(),
		ConversationID:    conversationID,
		SourceMessageID:   sourceMessageID,
		Provider:          providerOrDefault(assistantMessage.Provider),
		ProviderMessageID: strings.TrimSpace(assistantMessage.ProviderMessageID),
		ReplyType:         domain.ReplyTypeText,
		Body:              strings.TrimSpace(assistantMessage.Text),
		CreatedAt:         createdAt,
	}

	return s.assistantMessages.Create(ctx, &message)
}

func (s *CaptureService) persistInterpretation(ctx context.Context, farmID string, sourceMessage domain.SourceMessage, transcriptionID string, occurredAt time.Time) error {
	if s.interpreter == nil || s.interpretationRuns == nil {
		return nil
	}

	interpretation, err := s.interpreter.Interpret(ctx, InterpretationInput{
		MessageType: sourceMessage.MessageType,
		Text:        sourceMessage.RawText,
		OccurredAt:  occurredAt,
	})
	if err != nil {
		return err
	}
	if strings.TrimSpace(interpretation.NormalizedIntent) == "" {
		return nil
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
		return err
	}
	if s.businessEvents == nil {
		return nil
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

	return s.businessEvents.Create(ctx, &event)
}
