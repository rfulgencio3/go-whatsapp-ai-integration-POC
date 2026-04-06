package chatbot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

type ProcessResult struct {
	Duplicate bool
}

type Service struct {
	allowedPhoneNumber     string
	replyGenerator         ReplyGenerator
	messageSender          MessageSender
	incomingPreprocessor   IncomingMessagePreprocessor
	conversationRepository ConversationRepository
	messageArchive         MessageArchive
	messageDeduplicator    MessageDeduplicator
}

type noopMessageDeduplicator struct{}
type noopIncomingMessagePreprocessor struct{}

const unsupportedInboundReply = "No momento consigo processar mensagens de texto e audio no WhatsApp."

func NewService(
	allowedPhoneNumber string,
	replyGenerator ReplyGenerator,
	messageSender MessageSender,
	incomingPreprocessor IncomingMessagePreprocessor,
	conversationRepository ConversationRepository,
	messageArchive MessageArchive,
	messageDeduplicator MessageDeduplicator,
) *Service {
	if messageDeduplicator == nil {
		messageDeduplicator = noopMessageDeduplicator{}
	}
	if incomingPreprocessor == nil {
		incomingPreprocessor = noopIncomingMessagePreprocessor{}
	}

	return &Service{
		allowedPhoneNumber:     chat.NormalizePhoneNumber(allowedPhoneNumber),
		replyGenerator:         replyGenerator,
		messageSender:          messageSender,
		incomingPreprocessor:   incomingPreprocessor,
		conversationRepository: conversationRepository,
		messageArchive:         messageArchive,
		messageDeduplicator:    messageDeduplicator,
	}
}

func (noopMessageDeduplicator) Acquire(_ context.Context, _ string) (bool, error) { return true, nil }
func (noopMessageDeduplicator) MarkProcessed(_ context.Context, _ string) error   { return nil }
func (noopMessageDeduplicator) Release(_ context.Context, _ string) error         { return nil }
func (noopIncomingMessagePreprocessor) Prepare(_ context.Context, message chat.IncomingMessage) (chat.IncomingMessage, error) {
	return message, nil
}

func (s *Service) BuildReply(ctx context.Context, phoneNumber, userMessage string) (string, error) {
	normalizedPhoneNumber, userChatMessage, assistantChatMessage, err := s.buildReplyArtifacts(ctx, chat.IncomingMessage{
		PhoneNumber: phoneNumber,
		Text:        userMessage,
		Type:        chat.MessageTypeText,
	})
	if err != nil {
		return "", err
	}

	if err := s.persistConversation(ctx, normalizedPhoneNumber, userChatMessage, assistantChatMessage); err != nil {
		return "", err
	}

	return assistantChatMessage.Text, nil
}

func (s *Service) ProcessIncomingMessage(ctx context.Context, message chat.IncomingMessage) (ProcessResult, error) {
	normalizedMessage := normalizeIncomingMessage(message)

	if normalizedMessage.MessageID == "" {
		return ProcessResult{}, s.processAndSend(ctx, normalizedMessage)
	}

	acquired, err := s.messageDeduplicator.Acquire(ctx, normalizedMessage.MessageID)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("acquire message lock: %w", err)
	}
	if !acquired {
		return ProcessResult{Duplicate: true}, nil
	}

	if err := s.processAndSend(ctx, normalizedMessage); err != nil {
		if releaseErr := s.messageDeduplicator.Release(ctx, normalizedMessage.MessageID); releaseErr != nil {
			return ProcessResult{}, fmt.Errorf("%w (release message lock: %v)", err, releaseErr)
		}
		return ProcessResult{}, err
	}

	if err := s.messageDeduplicator.MarkProcessed(ctx, normalizedMessage.MessageID); err != nil {
		return ProcessResult{}, fmt.Errorf("mark message processed: %w", err)
	}

	return ProcessResult{}, nil
}

func (s *Service) processAndSend(ctx context.Context, incomingMessage chat.IncomingMessage) error {
	preparedMessage, err := s.incomingPreprocessor.Prepare(ctx, incomingMessage)
	if err != nil {
		if errors.Is(err, chat.ErrUnsupportedMessageType) {
			return s.sendUnsupportedReply(ctx, incomingMessage)
		}
		return err
	}
	if strings.TrimSpace(preparedMessage.Text) == "" {
		return s.sendUnsupportedReply(ctx, preparedMessage)
	}

	normalizedPhoneNumber, userChatMessage, assistantChatMessage, err := s.buildReplyArtifacts(ctx, preparedMessage)
	if err != nil {
		return err
	}

	if err := s.messageSender.SendTextMessage(ctx, normalizedPhoneNumber, assistantChatMessage.Text); err != nil {
		return err
	}

	return s.persistConversation(ctx, normalizedPhoneNumber, userChatMessage, assistantChatMessage)
}

func (s *Service) buildReplyArtifacts(ctx context.Context, incomingMessage chat.IncomingMessage) (string, chat.Message, chat.Message, error) {
	normalizedPhoneNumber := chat.NormalizePhoneNumber(incomingMessage.PhoneNumber)
	normalizedMessage := strings.TrimSpace(incomingMessage.Text)

	if s.allowedPhoneNumber != "" && normalizedPhoneNumber != s.allowedPhoneNumber {
		return "", chat.Message{}, chat.Message{}, chat.ErrPhoneNumberNotAllowed
	}

	history, err := s.conversationRepository.GetMessages(ctx, normalizedPhoneNumber)
	if err != nil {
		return "", chat.Message{}, chat.Message{}, err
	}

	userChatMessage := buildUserChatMessage(incomingMessage, normalizedMessage)
	historyForReply := append(append([]chat.Message(nil), history...), userChatMessage)

	reply, err := s.replyGenerator.GenerateReply(ctx, historyForReply)
	if err != nil {
		return "", chat.Message{}, chat.Message{}, err
	}

	assistantChatMessage := chat.Message{
		Role:      chat.AssistantRole,
		Text:      reply,
		CreatedAt: time.Now().UTC(),
		Type:      chat.MessageTypeText,
		Provider:  strings.TrimSpace(incomingMessage.Provider),
	}
	return normalizedPhoneNumber, userChatMessage, assistantChatMessage, nil
}

func (s *Service) sendUnsupportedReply(ctx context.Context, incomingMessage chat.IncomingMessage) error {
	normalizedPhoneNumber := chat.NormalizePhoneNumber(incomingMessage.PhoneNumber)
	if s.allowedPhoneNumber != "" && normalizedPhoneNumber != s.allowedPhoneNumber {
		return chat.ErrPhoneNumberNotAllowed
	}

	userChatMessage := buildUserChatMessage(incomingMessage, unsupportedInboundSummary(incomingMessage))
	assistantChatMessage := chat.Message{
		Role:      chat.AssistantRole,
		Text:      unsupportedInboundReply,
		CreatedAt: time.Now().UTC(),
		Type:      chat.MessageTypeText,
		Provider:  strings.TrimSpace(incomingMessage.Provider),
	}

	if err := s.messageSender.SendTextMessage(ctx, normalizedPhoneNumber, assistantChatMessage.Text); err != nil {
		return err
	}
	return s.persistConversation(ctx, normalizedPhoneNumber, userChatMessage, assistantChatMessage)
}

func (s *Service) persistConversation(ctx context.Context, phoneNumber string, userChatMessage, assistantChatMessage chat.Message) error {
	if err := s.conversationRepository.AppendMessage(ctx, phoneNumber, userChatMessage); err != nil {
		return err
	}
	if err := s.messageArchive.RecordMessage(ctx, phoneNumber, userChatMessage); err != nil {
		return err
	}
	if err := s.conversationRepository.AppendMessage(ctx, phoneNumber, assistantChatMessage); err != nil {
		return err
	}
	if err := s.messageArchive.RecordMessage(ctx, phoneNumber, assistantChatMessage); err != nil {
		return err
	}
	return nil
}

func normalizeIncomingMessage(message chat.IncomingMessage) chat.IncomingMessage {
	normalized := chat.IncomingMessage{
		MessageID:             strings.TrimSpace(message.MessageID),
		PhoneNumber:           chat.NormalizePhoneNumber(message.PhoneNumber),
		Text:                  strings.TrimSpace(message.Text),
		Type:                  message.Type,
		Provider:              strings.TrimSpace(message.Provider),
		TranscriptionID:       strings.TrimSpace(message.TranscriptionID),
		TranscriptionLanguage: strings.TrimSpace(message.TranscriptionLanguage),
		AudioDurationSeconds:  message.AudioDurationSeconds,
	}
	if normalized.Type == "" {
		if normalized.Text != "" {
			normalized.Type = chat.MessageTypeText
		} else {
			normalized.Type = chat.MessageTypeUnsupported
		}
	}
	if normalized.Provider == "" {
		normalized.Provider = "whatsapp"
	}

	normalized.MediaAttachments = make([]chat.MediaAttachment, 0, len(message.MediaAttachments))
	for _, attachment := range message.MediaAttachments {
		normalized.MediaAttachments = append(normalized.MediaAttachments, chat.MediaAttachment{
			URL:         strings.TrimSpace(attachment.URL),
			ContentType: strings.TrimSpace(attachment.ContentType),
			Filename:    strings.TrimSpace(attachment.Filename),
		})
	}

	return normalized
}

func buildUserChatMessage(incomingMessage chat.IncomingMessage, text string) chat.Message {
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

func unsupportedInboundSummary(incomingMessage chat.IncomingMessage) string {
	if incomingMessage.Type == "" {
		return "unsupported inbound message"
	}
	return fmt.Sprintf("unsupported inbound message of type %s", incomingMessage.Type)
}
