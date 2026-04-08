package agro

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

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
		Provider:          s.persistence.ProviderOrDefault(result.IncomingMessage.Provider),
		ProviderMessageID: strings.TrimSpace(result.IncomingMessage.MessageID),
		SenderPhoneNumber: phoneNumber,
		MessageType:       s.persistence.ToDomainMessageType(result.IncomingMessage.Type),
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
	transcriptionID, err := s.persistence.PersistTranscription(ctx, sourceMessage.ID, result.IncomingMessage, receivedAt)
	if err != nil {
		return err
	}
	event, requiresConfirmation, err := s.persistence.PersistInterpretation(ctx, membership.FarmID, sourceMessage, transcriptionID, receivedAt)
	if err != nil {
		return err
	}
	replyType := domain.ReplyTypeText
	if result.AssistantReplyKind == chatbot.ReplyKindConfirmation {
		replyType = domain.ReplyTypeConfirmation
	}
	if err := s.persistence.PersistAssistantMessage(ctx, conversation.ID, sourceMessage.ID, result.AssistantMessage, replyType, receivedAt); err != nil {
		return err
	}
	if requiresConfirmation {
		if err := s.conversations.SetPendingConfirmationEvent(ctx, conversation.ID, event.ID); err != nil {
			return err
		}
	}
	if strings.TrimSpace(conversation.PendingCorrectionEventID) != "" {
		if err := s.businessEvents.CreateCorrectionLink(ctx, event.ID, conversation.PendingCorrectionEventID); err != nil {
			return err
		}
		if err := s.conversations.SetPendingCorrectionEvent(ctx, conversation.ID, ""); err != nil {
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
		Text:      s.replyFormatter.BuildDraftConfirmationPrompt(event),
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
	if err := s.persistence.PersistAssistantMessage(ctx, conversationID, sourceMessageID, assistantMessage, domain.ReplyTypeConfirmation, createdAt); err != nil {
		return err
	}

	return nil
}

func (s *CaptureService) handleConfirmationMessage(ctx context.Context, membership domain.FarmMembership, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	decision := s.workflowRouter.ClassifyConfirmationDecision(message.Text)
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
	replyText := s.replyFormatter.BuildRejectedReply()
	if decision == confirmationAccepted {
		status = domain.EventStatusConfirmed
		confirmedByUser = true
		replyText = s.replyFormatter.BuildConfirmedReply(event)
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
	if decision == confirmationRejected {
		if err := s.conversations.SetPendingCorrectionEvent(ctx, conversation.ID, event.ID); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
	} else {
		if err := s.conversations.SetPendingCorrectionEvent(ctx, conversation.ID, ""); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
	}
	if confirmedByUser {
		replyText, err = s.preparePostConfirmationReply(ctx, normalizedPhone, membership, event, replyText, now)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
	}

	savedConversation, sourceMessage, err := s.persistConfirmationInbound(ctx, membership, message, now)
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}

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
	if err := s.persistence.PersistAssistantMessage(ctx, savedConversation.ID, sourceMessage.ID, assistantMessage, domain.ReplyTypeConfirmation, now); err != nil {
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
		Provider:          s.persistence.ProviderOrDefault(message.Provider),
		ProviderMessageID: strings.TrimSpace(message.MessageID),
		SenderPhoneNumber: phoneNumber,
		MessageType:       s.persistence.ToDomainMessageType(message.Type),
		RawText:           strings.TrimSpace(message.Text),
		ReceivedAt:        receivedAt,
		CreatedAt:         receivedAt,
	}
	if err := s.sourceMessages.Create(ctx, &sourceMessage); err != nil {
		return domain.Conversation{}, domain.SourceMessage{}, err
	}

	return conversation, sourceMessage, nil
}

func (s *CaptureService) preparePostConfirmationReply(ctx context.Context, phoneNumber string, membership domain.FarmMembership, event domain.BusinessEvent, defaultReply string, now time.Time) (string, error) {
	if s.correlatedStates == nil || event.Category != "health" {
		return defaultReply, nil
	}

	state := domain.CorrelatedExpenseState{
		PhoneNumber:     phoneNumber,
		FarmID:          membership.FarmID,
		RootEventID:     event.ID,
		RootCategory:    event.Category,
		RootSubcategory: event.Subcategory,
		AnimalCode:      event.AnimalCode,
		Description:     event.Description,
		OccurredAt:      event.OccurredAt,
		Step:            domain.CorrelatedExpenseStepAwaitingDecision,
		UpdatedAt:       now,
	}
	if err := s.correlatedStates.Upsert(ctx, &state); err != nil {
		return "", err
	}

	return s.replyFormatter.BuildHealthExpenseCorrelationPrompt(event), nil
}
