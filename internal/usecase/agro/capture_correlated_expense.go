package agro

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type defaultCorrelatedExpenseFlow struct {
	service *CaptureService
}

func newDefaultCorrelatedExpenseFlow(service *CaptureService) CorrelatedExpenseFlow {
	if service == nil {
		return nil
	}
	return &defaultCorrelatedExpenseFlow{service: service}
}

func (f *defaultCorrelatedExpenseFlow) HandleIncomingMessage(ctx context.Context, membership domain.FarmMembership, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	s := f.service
	if s == nil || s.correlatedStates == nil || s.messageSender == nil {
		return false, chatbot.ProcessResult{}, nil
	}

	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)
	state, found, err := s.correlatedStates.GetByPhoneNumber(ctx, normalizedPhone)
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if !found || state.FarmID != membership.FarmID {
		return false, chatbot.ProcessResult{}, nil
	}

	text := strings.TrimSpace(message.Text)
	if text == "" {
		return true, chatbot.ProcessResult{}, nil
	}

	switch state.Step {
	case domain.CorrelatedExpenseStepAwaitingDecision:
		decision := s.workflowRouter.ClassifyConfirmationDecision(text)
		switch decision {
		case confirmationAccepted:
			state.Step = domain.CorrelatedExpenseStepAwaitingMedicineAmount
			state.UpdatedAt = time.Now().UTC()
			if err := s.correlatedStates.Upsert(ctx, &state); err != nil {
				return false, chatbot.ProcessResult{}, err
			}
			return f.sendStateReply(ctx, message, state)
		case confirmationRejected:
			if err := s.correlatedStates.DeleteByPhoneNumber(ctx, normalizedPhone); err != nil {
				return false, chatbot.ProcessResult{}, err
			}
			return f.sendReply(ctx, message, s.replyFormatter.BuildCorrelatedExpenseDeclinedReply())
		default:
			return f.sendReply(ctx, message, "Responda SIM para lancar os gastos ou NAO para pular essa etapa.")
		}
	case domain.CorrelatedExpenseStepAwaitingMedicineAmount:
		value, ok := parseExpenseAmount(text)
		if !ok {
			return f.sendReply(ctx, message, "Informe o valor do medicamento. Se nao houve, responda 0.")
		}
		state.MedicineAmount = &value
		state.Step = domain.CorrelatedExpenseStepAwaitingVetAmount
		state.UpdatedAt = time.Now().UTC()
		if err := s.correlatedStates.Upsert(ctx, &state); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		return f.sendStateReply(ctx, message, state)
	case domain.CorrelatedExpenseStepAwaitingVetAmount:
		value, ok := parseExpenseAmount(text)
		if !ok {
			return f.sendReply(ctx, message, "Informe o valor da consulta veterinaria. Se nao houve, responda 0.")
		}
		state.VetAmount = &value
		state.Step = domain.CorrelatedExpenseStepAwaitingExamAmount
		state.UpdatedAt = time.Now().UTC()
		if err := s.correlatedStates.Upsert(ctx, &state); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		return f.sendStateReply(ctx, message, state)
	case domain.CorrelatedExpenseStepAwaitingExamAmount:
		value, ok := parseExpenseAmount(text)
		if !ok {
			return f.sendReply(ctx, message, "Informe o valor de exames. Se nao houve, responda 0.")
		}
		state.ExamAmount = &value
		state.UpdatedAt = time.Now().UTC()
		return f.finalizeFlow(ctx, membership, state, message)
	default:
		return false, chatbot.ProcessResult{}, nil
	}
}

func (f *defaultCorrelatedExpenseFlow) sendStateReply(ctx context.Context, message chat.IncomingMessage, state domain.CorrelatedExpenseState) (bool, chatbot.ProcessResult, error) {
	return f.sendReply(ctx, message, f.service.replyFormatter.BuildCorrelatedExpenseQuestion(state))
}

func (f *defaultCorrelatedExpenseFlow) sendReply(ctx context.Context, message chat.IncomingMessage, replyText string) (bool, chatbot.ProcessResult, error) {
	s := f.service
	now := time.Now().UTC()
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

	return true, chatbot.ProcessResult{
		PhoneNumber:      normalizedPhone,
		IncomingMessage:  message,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
	}, nil
}

func (f *defaultCorrelatedExpenseFlow) finalizeFlow(ctx context.Context, membership domain.FarmMembership, state domain.CorrelatedExpenseState, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	s := f.service
	now := time.Now().UTC()
	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)

	conversation, err := s.conversations.GetOrCreateOpen(ctx, membership.FarmID, "whatsapp", normalizedPhone, now)
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	sourceText := buildCorrelatedExpenseSourceText(state)
	sourceMessage := domain.SourceMessage{
		ID:                uuid.NewString(),
		ConversationID:    conversation.ID,
		Provider:          s.persistence.ProviderOrDefault(message.Provider),
		ProviderMessageID: strings.TrimSpace(message.MessageID),
		SenderPhoneNumber: normalizedPhone,
		MessageType:       s.persistence.ToDomainMessageType(chat.MessageTypeText),
		RawText:           sourceText,
		ReceivedAt:        now,
		CreatedAt:         now,
	}
	if err := s.sourceMessages.Create(ctx, &sourceMessage); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	for _, result := range buildCorrelatedExpenseInterpretations(state) {
		run := domain.InterpretationRun{
			ID:                   uuid.NewString(),
			SourceMessageID:      sourceMessage.ID,
			ModelProvider:        interpreterProvider,
			ModelName:            interpreterModel,
			PromptVersion:        promptVersion,
			NormalizedIntent:     result.NormalizedIntent,
			Confidence:           result.Confidence,
			RequiresConfirmation: false,
			RawOutputJSON:        buildInterpretationPayload(result),
			CreatedAt:            now,
		}
		if err := s.interpretationRuns.Create(ctx, &run); err != nil {
			return false, chatbot.ProcessResult{}, err
		}

		event := domain.BusinessEvent{
			ID:                  uuid.NewString(),
			FarmID:              membership.FarmID,
			SourceMessageID:     sourceMessage.ID,
			InterpretationRunID: run.ID,
			Category:            result.Category,
			Subcategory:         result.Subcategory,
			OccurredAt:          state.OccurredAt,
			Description:         result.Description,
			Amount:              result.Amount,
			Currency:            result.Currency,
			AnimalCode:          state.AnimalCode,
			Status:              domain.EventStatusConfirmed,
			ConfirmedByUser:     true,
			ConfirmedAt:         &now,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if err := s.businessEvents.Create(ctx, &event); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		if err := s.businessEvents.CreateAttributes(ctx, event.ID, result.Attributes); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
	}

	replyText := s.replyFormatter.BuildCorrelatedExpenseRecordedReply(state)
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
	if err := s.persistence.PersistAssistantMessage(ctx, conversation.ID, sourceMessage.ID, assistantMessage, domain.ReplyTypeText, now); err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if err := s.correlatedStates.DeleteByPhoneNumber(ctx, normalizedPhone); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	return true, chatbot.ProcessResult{
		PhoneNumber:      normalizedPhone,
		IncomingMessage:  message,
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
	}, nil
}

func buildCorrelatedExpenseSourceText(state domain.CorrelatedExpenseState) string {
	parts := []string{"gastos relacionados ao tratamento"}
	if strings.TrimSpace(state.AnimalCode) != "" {
		parts = append(parts, "animal: "+state.AnimalCode)
	}
	if state.MedicineAmount != nil {
		parts = append(parts, "medicamento: "+strconv.FormatFloat(*state.MedicineAmount, 'f', 2, 64))
	}
	if state.VetAmount != nil {
		parts = append(parts, "consulta veterinaria: "+strconv.FormatFloat(*state.VetAmount, 'f', 2, 64))
	}
	if state.ExamAmount != nil {
		parts = append(parts, "exames: "+strconv.FormatFloat(*state.ExamAmount, 'f', 2, 64))
	}
	return strings.Join(parts, "; ")
}

func buildCorrelatedExpenseInterpretations(state domain.CorrelatedExpenseState) []InterpretationResult {
	type expenseEntry struct {
		expenseType string
		label       string
		amount      *float64
	}
	entries := []expenseEntry{
		{expenseType: "medicine", label: "medicamento", amount: state.MedicineAmount},
		{expenseType: "vet_consultation", label: "consulta veterinaria", amount: state.VetAmount},
		{expenseType: "exam", label: "exames", amount: state.ExamAmount},
	}

	results := make([]InterpretationResult, 0, len(entries))
	for _, entry := range entries {
		if entry.amount == nil || *entry.amount <= 0 {
			continue
		}
		description := "Gasto com " + entry.label + " relacionado ao tratamento"
		if strings.TrimSpace(state.AnimalCode) != "" {
			description += " do animal " + state.AnimalCode
		}
		results = append(results, InterpretationResult{
			NormalizedIntent:     "finance.expense",
			Category:             "finance",
			Subcategory:          "expense",
			Description:          description,
			AnimalCode:           state.AnimalCode,
			Confidence:           1,
			RequiresConfirmation: false,
			Amount:               entry.amount,
			Currency:             "BRL",
			OccurredAt:           state.OccurredAt,
			Attributes: map[string]string{
				"related_event_id": state.RootEventID,
				"expense_type":     entry.expenseType,
			},
		})
	}
	return results
}

func parseExpenseAmount(text string) (float64, bool) {
	normalized := normalizeText(text)
	if normalized == "" {
		return 0, false
	}
	if amount := extractAmount(text, nil); amount != nil {
		return *amount, true
	}
	for _, token := range strings.Fields(normalized) {
		if value, ok := parseDecimal(token); ok {
			return value, true
		}
	}
	return 0, false
}
