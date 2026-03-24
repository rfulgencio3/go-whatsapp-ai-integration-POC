package chatbot

import (
	"context"
	"strings"
	"time"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

type Service struct {
	allowedPhoneNumber     string
	replyGenerator         ReplyGenerator
	messageSender          MessageSender
	conversationRepository ConversationRepository
	messageArchive         MessageArchive
}

func NewService(
	allowedPhoneNumber string,
	replyGenerator ReplyGenerator,
	messageSender MessageSender,
	conversationRepository ConversationRepository,
	messageArchive MessageArchive,
) *Service {
	return &Service{
		allowedPhoneNumber:     chat.NormalizePhoneNumber(allowedPhoneNumber),
		replyGenerator:         replyGenerator,
		messageSender:          messageSender,
		conversationRepository: conversationRepository,
		messageArchive:         messageArchive,
	}
}

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

	userChatMessage := chat.Message{
		Role:      chat.UserRole,
		Text:      normalizedMessage,
		CreatedAt: time.Now().UTC(),
	}

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

	assistantChatMessage := chat.Message{
		Role:      chat.AssistantRole,
		Text:      reply,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.conversationRepository.AppendMessage(ctx, normalizedPhoneNumber, assistantChatMessage); err != nil {
		return "", err
	}

	if err := s.messageArchive.RecordMessage(ctx, normalizedPhoneNumber, assistantChatMessage); err != nil {
		return "", err
	}

	return reply, nil
}

func (s *Service) ProcessIncomingMessage(ctx context.Context, message chat.IncomingMessage) error {
	reply, err := s.BuildReply(ctx, message.PhoneNumber, message.Text)
	if err != nil {
		return err
	}

	return s.messageSender.SendTextMessage(ctx, chat.NormalizePhoneNumber(message.PhoneNumber), reply)
}
