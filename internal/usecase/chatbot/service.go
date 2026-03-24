package chatbot

import (
	"context"
	"strings"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

type Service struct {
	allowedPhoneNumber     string
	replyGenerator         ReplyGenerator
	messageSender          MessageSender
	conversationRepository ConversationRepository
}

func NewService(
	allowedPhoneNumber string,
	replyGenerator ReplyGenerator,
	messageSender MessageSender,
	conversationRepository ConversationRepository,
) *Service {
	return &Service{
		allowedPhoneNumber:     chat.NormalizePhoneNumber(allowedPhoneNumber),
		replyGenerator:         replyGenerator,
		messageSender:          messageSender,
		conversationRepository: conversationRepository,
	}
}

func (s *Service) BuildReply(ctx context.Context, phoneNumber, userMessage string) (string, error) {
	normalizedPhoneNumber := chat.NormalizePhoneNumber(phoneNumber)
	normalizedMessage := strings.TrimSpace(userMessage)

	if s.allowedPhoneNumber != "" && normalizedPhoneNumber != s.allowedPhoneNumber {
		return "", chat.ErrPhoneNumberNotAllowed
	}

	history := s.conversationRepository.AppendMessage(normalizedPhoneNumber, chat.Message{
		Role: chat.UserRole,
		Text: normalizedMessage,
	})

	reply, err := s.replyGenerator.GenerateReply(ctx, history)
	if err != nil {
		return "", err
	}

	s.conversationRepository.AppendMessage(normalizedPhoneNumber, chat.Message{
		Role: chat.AssistantRole,
		Text: reply,
	})

	return reply, nil
}

func (s *Service) ProcessIncomingMessage(ctx context.Context, message chat.IncomingMessage) error {
	reply, err := s.BuildReply(ctx, message.PhoneNumber, message.Text)
	if err != nil {
		return err
	}

	return s.messageSender.SendTextMessage(ctx, chat.NormalizePhoneNumber(message.PhoneNumber), reply)
}
