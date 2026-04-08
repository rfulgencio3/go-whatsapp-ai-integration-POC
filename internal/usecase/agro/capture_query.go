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

type defaultBusinessQueryFlow struct {
	service *CaptureService
}

func newDefaultBusinessQueryFlow(service *CaptureService) BusinessQueryFlow {
	if service == nil {
		return nil
	}
	return &defaultBusinessQueryFlow{service: service}
}

func (f *defaultBusinessQueryFlow) HandleIncomingMessage(ctx context.Context, membership domain.FarmMembership, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	s := f.service
	if s == nil || s.workflowRouter == nil || s.replyFormatter == nil || s.businessEvents == nil || s.messageSender == nil {
		return false, chatbot.ProcessResult{}, nil
	}
	now := time.Now().UTC()
	var (
		replyText string
		err       error
	)
	switch {
	case s.workflowRouter.IsMilkWithdrawalQuery(message.Text):
		var items []domain.MilkWithdrawalAnimal
		items, err = s.businessEvents.ListActiveMilkWithdrawalAnimals(ctx, membership.FarmID, now)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		replyText = s.replyFormatter.BuildMilkWithdrawalQueryReply(items, now)
	case s.workflowRouter.IsRecentTreatmentsQuery(message.Text):
		var items []domain.HealthTreatmentSummary
		items, err = s.businessEvents.ListRecentHealthTreatments(ctx, membership.FarmID, 5)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		replyText = s.replyFormatter.BuildRecentHealthTreatmentsReply(items, now)
	case s.workflowRouter.IsMedicineExpenseMonthQuery(message.Text):
		periodStart, periodEnd := currentMonthRange(now)
		var amount float64
		amount, err = s.businessEvents.SumMedicineExpensesForMonth(ctx, membership.FarmID, periodStart, periodEnd)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		replyText = s.replyFormatter.BuildMedicineExpenseMonthReply(amount, now)
	case s.workflowRouter.IsVetExpenseMonthQuery(message.Text):
		periodStart, periodEnd := currentMonthRange(now)
		var amount float64
		amount, err = s.businessEvents.SumVetExpensesForMonth(ctx, membership.FarmID, periodStart, periodEnd)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		replyText = s.replyFormatter.BuildVetExpenseMonthReply(amount, now)
	case s.workflowRouter.IsRecentPurchasesQuery(message.Text):
		var items []domain.InputPurchaseSummary
		items, err = s.businessEvents.ListRecentInputPurchases(ctx, membership.FarmID, 5)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		replyText = s.replyFormatter.BuildRecentInputPurchasesReply(items, now)
	default:
		return false, chatbot.ProcessResult{}, nil
	}

	return f.sendQueryReply(ctx, membership, message, replyText, now)
}

func (f *defaultBusinessQueryFlow) sendQueryReply(ctx context.Context, membership domain.FarmMembership, message chat.IncomingMessage, replyText string, now time.Time) (bool, chatbot.ProcessResult, error) {
	s := f.service
	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)
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

	if s.conversations != nil && s.sourceMessages != nil {
		conversation, err := s.conversations.GetOrCreateOpen(ctx, membership.FarmID, "whatsapp", normalizedPhone, now)
		if err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		sourceMessage := domain.SourceMessage{
			ID:                uuid.NewString(),
			ConversationID:    conversation.ID,
			Provider:          s.persistence.ProviderOrDefault(message.Provider),
			ProviderMessageID: strings.TrimSpace(message.MessageID),
			SenderPhoneNumber: normalizedPhone,
			MessageType:       s.persistence.ToDomainMessageType(message.Type),
			RawText:           strings.TrimSpace(message.Text),
			ReceivedAt:        now,
			CreatedAt:         now,
		}
		if err := s.sourceMessages.Create(ctx, &sourceMessage); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		if err := s.persistence.PersistAssistantMessage(ctx, conversation.ID, sourceMessage.ID, assistantMessage, domain.ReplyTypeText, now); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
	}

	return true, chatbot.ProcessResult{
		PhoneNumber:      normalizedPhone,
		IncomingMessage:  message,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
	}, nil
}

func currentMonthRange(reference time.Time) (time.Time, time.Time) {
	location := time.FixedZone("BRT", -3*60*60)
	local := reference.In(location)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
	end := start.AddDate(0, 1, 0)
	return start.UTC(), end.UTC()
}
