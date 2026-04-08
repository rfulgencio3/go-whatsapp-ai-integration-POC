package agro

import (
	"context"
	"strings"
	"time"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

func (s *CaptureService) handleHelpRequest(ctx context.Context, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	if s.messageSender == nil || s.workflowRouter == nil || !s.workflowRouter.IsHelpCommand(message.Text) {
		return false, chatbot.ProcessResult{}, nil
	}

	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)
	registered := false
	if s.farmMemberships != nil {
		memberships, err := s.farmMemberships.FindActiveByPhoneNumber(ctx, normalizedPhone)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		registered = len(memberships) > 0
	}

	replyText := s.replyFormatter.BuildHelpReply(registered)
	now := time.Now().UTC()
	if err := s.messageSender.SendTextMessage(ctx, normalizedPhone, replyText); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	userMessage := s.persistence.BuildChatMessageFromIncoming(message, strings.TrimSpace(message.Text))
	assistantMessage := chat.Message{
		Role:      chat.AssistantRole,
		Text:      replyText,
		CreatedAt: now,
		Type:      chat.MessageTypeText,
		Provider:  s.persistence.ProviderOrDefault(message.Provider),
	}
	if err := s.persistence.PersistLegacyConversation(ctx, normalizedPhone, userMessage, assistantMessage); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	return true, chatbot.ProcessResult{
		PhoneNumber:      normalizedPhone,
		IncomingMessage:  message,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
	}, nil
}
