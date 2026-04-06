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
	logger          *observability.Logger
	downstream      chatbot.MessageProcessor
	farmMemberships FarmMembershipRepository
	conversations   ConversationRepository
	sourceMessages  SourceMessageRepository
}

func NewCaptureService(
	logger *observability.Logger,
	downstream chatbot.MessageProcessor,
	farmMemberships FarmMembershipRepository,
	conversations ConversationRepository,
	sourceMessages SourceMessageRepository,
) *CaptureService {
	if logger == nil {
		logger = observability.NewLogger()
	}

	return &CaptureService{
		logger:          logger,
		downstream:      downstream,
		farmMemberships: farmMemberships,
		conversations:   conversations,
		sourceMessages:  sourceMessages,
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

	if err := s.captureInboundMessage(ctx, message); err != nil {
		s.logger.Error("agro inbound capture failed", map[string]any{
			"phone_number": domain.NormalizePhoneNumber(message.PhoneNumber),
			"message_id":   strings.TrimSpace(message.MessageID),
			"provider":     strings.TrimSpace(message.Provider),
			"error":        err.Error(),
		})
	}

	return result, nil
}

func (s *CaptureService) captureInboundMessage(ctx context.Context, message chat.IncomingMessage) error {
	if s.farmMemberships == nil || s.conversations == nil || s.sourceMessages == nil {
		return nil
	}

	phoneNumber := domain.NormalizePhoneNumber(message.PhoneNumber)
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
			"provider":     strings.TrimSpace(message.Provider),
		})
		return nil
	case 1:
	default:
		s.logger.Info("agro context is ambiguous for inbound phone", map[string]any{
			"phone_number":      phoneNumber,
			"provider":          strings.TrimSpace(message.Provider),
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
		Provider:          providerOrDefault(message.Provider),
		ProviderMessageID: strings.TrimSpace(message.MessageID),
		SenderPhoneNumber: phoneNumber,
		MessageType:       toDomainMessageType(message.Type),
		RawText:           strings.TrimSpace(message.Text),
		ReceivedAt:        receivedAt,
		CreatedAt:         receivedAt,
	}
	if len(message.MediaAttachments) > 0 {
		sourceMessage.MediaURL = strings.TrimSpace(message.MediaAttachments[0].URL)
		sourceMessage.MediaContentType = strings.TrimSpace(message.MediaAttachments[0].ContentType)
		sourceMessage.MediaFilename = strings.TrimSpace(message.MediaAttachments[0].Filename)
	}

	return s.sourceMessages.Create(ctx, &sourceMessage)
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
