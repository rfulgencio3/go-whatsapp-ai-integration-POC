package chatbot

import (
	"context"
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
	conversationRepository ConversationRepository
	messageArchive         MessageArchive
	messageDeduplicator    MessageDeduplicator
}

type noopMessageDeduplicator struct{}

func NewService(
	allowedPhoneNumber string,
	replyGenerator ReplyGenerator,
	messageSender MessageSender,
	conversationRepository ConversationRepository,
	messageArchive MessageArchive,
	messageDeduplicator MessageDeduplicator,
) *Service {
	if messageDeduplicator == nil {
		messageDeduplicator = noopMessageDeduplicator{}
	}

	return &Service{
		allowedPhoneNumber:     chat.NormalizePhoneNumber(allowedPhoneNumber),
		replyGenerator:         replyGenerator,
		messageSender:          messageSender,
		conversationRepository: conversationRepository,
		messageArchive:         messageArchive,
		messageDeduplicator:    messageDeduplicator,
	}
}

func (noopMessageDeduplicator) Acquire(_ context.Context, _ string) (bool, error) { return true, nil }
func (noopMessageDeduplicator) MarkProcessed(_ context.Context, _ string) error   { return nil }
func (noopMessageDeduplicator) Release(_ context.Context, _ string) error         { return nil }

func (s *Service) BuildReply(ctx context.Context, phoneNumber, userMessage string) (string, error) {
	normalizedPhoneNumber := chat.NormalizePhoneNumber(phoneNumber)
	normalizedMessage := strings.TrimSpace(userMessage)

	if s.allowedPhoneNumber != "" && normalizedPhoneNumber != s.allowedPhoneNumber {
		return "", chat.ErrPhoneNumberNotAllowed
	}

	history, err := s.conversationRepository.GetMessages(ctx, normalizedPhoneNumber)
	if err != nil {
		return "", err
	}

	userChatMessage := chat.Message{Role: chat.UserRole, Text: normalizedMessage, CreatedAt: time.Now().UTC()}
	if err := s.conversationRepository.AppendMessage(ctx, normalizedPhoneNumber, userChatMessage); err != nil {
		return "", err
	}
	if err := s.messageArchive.RecordMessage(ctx, normalizedPhoneNumber, userChatMessage); err != nil {
		return "", err
	}

	history = append(history, userChatMessage)
	reply, err := s.replyGenerator.GenerateReply(ctx, history)
	if err != nil {
		return "", err
	}

	assistantChatMessage := chat.Message{Role: chat.AssistantRole, Text: reply, CreatedAt: time.Now().UTC()}
	if err := s.conversationRepository.AppendMessage(ctx, normalizedPhoneNumber, assistantChatMessage); err != nil {
		return "", err
	}
	if err := s.messageArchive.RecordMessage(ctx, normalizedPhoneNumber, assistantChatMessage); err != nil {
		return "", err
	}

	return reply, nil
}

func (s *Service) ProcessIncomingMessage(ctx context.Context, message chat.IncomingMessage) (ProcessResult, error) {
	normalizedMessage := chat.IncomingMessage{MessageID: strings.TrimSpace(message.MessageID), PhoneNumber: chat.NormalizePhoneNumber(message.PhoneNumber), Text: strings.TrimSpace(message.Text)}

	if normalizedMessage.MessageID == "" {
		return ProcessResult{}, s.processAndSend(ctx, normalizedMessage.PhoneNumber, normalizedMessage.Text)
	}

	acquired, err := s.messageDeduplicator.Acquire(ctx, normalizedMessage.MessageID)
	if err != nil {
		return ProcessResult{}, fmt.Errorf("acquire message lock: %w", err)
	}
	if !acquired {
		return ProcessResult{Duplicate: true}, nil
	}

	if err := s.processAndSend(ctx, normalizedMessage.PhoneNumber, normalizedMessage.Text); err != nil {
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

func (s *Service) processAndSend(ctx context.Context, phoneNumber, userMessage string) error {
	reply, err := s.BuildReply(ctx, phoneNumber, userMessage)
	if err != nil {
		return err
	}
	return s.messageSender.SendTextMessage(ctx, chat.NormalizePhoneNumber(phoneNumber), reply)
}
