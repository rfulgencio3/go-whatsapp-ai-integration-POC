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
	if s.onboardingStates != nil {
		state, found, err := s.onboardingStates.GetByPhoneNumber(ctx, normalizedPhone)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		if found {
			return s.sendHelpReply(ctx, message, normalizedPhone, s.replyFormatter.BuildOnboardingHelpReply(state.Step))
		}
	}

	if s.healthStates != nil {
		state, found, err := s.healthStates.GetByPhoneNumber(ctx, normalizedPhone)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		if found {
			return s.sendHelpReply(ctx, message, normalizedPhone, s.replyFormatter.BuildHealthTreatmentHelpReply(state))
		}
	}

	if s.correlatedStates != nil {
		state, found, err := s.correlatedStates.GetByPhoneNumber(ctx, normalizedPhone)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		if found {
			return s.sendHelpReply(ctx, message, normalizedPhone, s.replyFormatter.BuildCorrelatedExpenseHelpReply(state))
		}
	}

	registered := false
	if s.farmMemberships != nil {
		memberships, err := s.farmMemberships.FindActiveByPhoneNumber(ctx, normalizedPhone)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		registered = len(memberships) > 0
	}

	return s.sendHelpReply(ctx, message, normalizedPhone, s.replyFormatter.BuildHelpReply(s.workflowRouter.ParseHelpTopic(message.Text), registered))
}

func (s *CaptureService) sendHelpReply(ctx context.Context, message chat.IncomingMessage, normalizedPhone, replyText string) (bool, chatbot.ProcessResult, error) {
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
