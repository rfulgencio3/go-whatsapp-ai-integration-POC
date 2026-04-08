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

type defaultHealthTreatmentFlow struct {
	service *CaptureService
}

func newDefaultHealthTreatmentFlow(service *CaptureService) HealthTreatmentFlow {
	if service == nil {
		return nil
	}
	return &defaultHealthTreatmentFlow{service: service}
}

func (f *defaultHealthTreatmentFlow) HandleIncomingMessage(ctx context.Context, membership domain.FarmMembership, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	s := f.service
	if s == nil || s.healthStates == nil || s.messageSender == nil || s.interpreter == nil {
		return false, chatbot.ProcessResult{}, nil
	}

	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)
	state, found, err := s.healthStates.GetByPhoneNumber(ctx, normalizedPhone)
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	if found && state.FarmID == membership.FarmID {
		return f.continueFlow(ctx, membership, state, message)
	}

	interpretation, err := s.interpreter.Interpret(ctx, InterpretationInput{
		MessageType: s.persistence.ToDomainMessageType(message.Type),
		Text:        strings.TrimSpace(message.Text),
		OccurredAt:  time.Now().UTC(),
	})
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if interpretation.Category != "health" || strings.TrimSpace(interpretation.AnimalCode) == "" {
		return false, chatbot.ProcessResult{}, nil
	}

	state = domain.HealthTreatmentState{
		PhoneNumber: normalizedPhone,
		FarmID:      membership.FarmID,
		Category:    interpretation.Category,
		Subcategory: interpretation.Subcategory,
		AnimalCode:  strings.TrimSpace(interpretation.AnimalCode),
		Description: strings.TrimSpace(interpretation.Description),
		Attributes:  cloneAttributes(interpretation.Attributes),
		UpdatedAt:   time.Now().UTC(),
	}
	if interpretation.OccurredAt != nil {
		state.DiagnosisOccurredAt = interpretation.OccurredAt
		state.DiagnosisDateText = formatHealthDiagnosisDate(*interpretation.OccurredAt)
		state.Step = domain.HealthTreatmentStepAwaitingMedicine
	} else {
		state.Step = domain.HealthTreatmentStepAwaitingDiagnosisDate
	}

	if err := s.healthStates.Upsert(ctx, &state); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	return f.sendStateReply(ctx, message, state)
}

func (f *defaultHealthTreatmentFlow) continueFlow(ctx context.Context, membership domain.FarmMembership, state domain.HealthTreatmentState, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	s := f.service
	text := strings.TrimSpace(message.Text)
	if text == "" {
		return true, chatbot.ProcessResult{}, nil
	}

	switch state.Step {
	case domain.HealthTreatmentStepAwaitingDiagnosisDate:
		state.DiagnosisDateText = normalizeHealthAnswer(text)
		if occurredAt, ok := parseDiagnosisDate(text); ok {
			state.DiagnosisOccurredAt = &occurredAt
		}
		state.Step = domain.HealthTreatmentStepAwaitingMedicine
		state.UpdatedAt = time.Now().UTC()
		if err := s.healthStates.Upsert(ctx, &state); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		return f.sendStateReply(ctx, message, state)
	case domain.HealthTreatmentStepAwaitingMedicine:
		state.Medicine = normalizeHealthAnswer(text)
		if state.Medicine == "" {
			return f.sendValidationReply(ctx, message, "Informe o medicamento para continuar.")
		}
		state.Step = domain.HealthTreatmentStepAwaitingTreatmentDays
		state.UpdatedAt = time.Now().UTC()
		if err := s.healthStates.Upsert(ctx, &state); err != nil {
			return false, chatbot.ProcessResult{}, err
		}
		return f.sendStateReply(ctx, message, state)
	case domain.HealthTreatmentStepAwaitingTreatmentDays:
		days, ok := parseTreatmentDays(text)
		if !ok {
			return f.sendValidationReply(ctx, message, "Informe a quantidade de dias de tratamento.")
		}
		state.TreatmentDays = days
		state.UpdatedAt = time.Now().UTC()
		return f.finalizeFlow(ctx, membership, state, message)
	default:
		return false, chatbot.ProcessResult{}, nil
	}
}

func (f *defaultHealthTreatmentFlow) finalizeFlow(ctx context.Context, membership domain.FarmMembership, state domain.HealthTreatmentState, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error) {
	s := f.service
	now := time.Now().UTC()
	normalizedPhone := domain.NormalizePhoneNumber(message.PhoneNumber)

	conversation, err := s.conversations.GetOrCreateOpen(ctx, membership.FarmID, "whatsapp", normalizedPhone, now)
	if err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	sourceText := buildHealthTreatmentSourceText(state)
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

	interpretation := buildHealthInterpretationResult(state)
	run := domain.InterpretationRun{
		ID:                   uuid.NewString(),
		SourceMessageID:      sourceMessage.ID,
		ModelProvider:        interpreterProvider,
		ModelName:            interpreterModel,
		PromptVersion:        promptVersion,
		NormalizedIntent:     interpretation.NormalizedIntent,
		Confidence:           interpretation.Confidence,
		RequiresConfirmation: true,
		RawOutputJSON:        buildInterpretationPayload(interpretation),
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
		Category:            interpretation.Category,
		Subcategory:         interpretation.Subcategory,
		OccurredAt:          state.DiagnosisOccurredAt,
		Description:         interpretation.Description,
		AnimalCode:          interpretation.AnimalCode,
		Status:              domain.EventStatusDraft,
		ConfirmedByUser:     false,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := s.businessEvents.Create(ctx, &event); err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if err := s.businessEvents.CreateAttributes(ctx, event.ID, interpretation.Attributes); err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if err := s.conversations.SetPendingConfirmationEvent(ctx, conversation.ID, event.ID); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	replyText := s.replyFormatter.BuildDraftConfirmationPromptFromInterpretation(interpretation)
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
	if err := s.persistence.PersistAssistantMessage(ctx, conversation.ID, sourceMessage.ID, assistantMessage, domain.ReplyTypeConfirmation, now); err != nil {
		return false, chatbot.ProcessResult{}, err
	}
	if err := s.healthStates.DeleteByPhoneNumber(ctx, normalizedPhone); err != nil {
		return false, chatbot.ProcessResult{}, err
	}

	return true, chatbot.ProcessResult{
		PhoneNumber:        normalizedPhone,
		IncomingMessage:    message,
		UserMessage:        userMessage,
		AssistantMessage:   assistantMessage,
		AssistantReplyKind: chatbot.ReplyKindConfirmation,
	}, nil
}

func (f *defaultHealthTreatmentFlow) sendStateReply(ctx context.Context, message chat.IncomingMessage, state domain.HealthTreatmentState) (bool, chatbot.ProcessResult, error) {
	return f.sendValidationReply(ctx, message, f.service.replyFormatter.BuildHealthTreatmentQuestion(state))
}

func (f *defaultHealthTreatmentFlow) sendValidationReply(ctx context.Context, message chat.IncomingMessage, replyText string) (bool, chatbot.ProcessResult, error) {
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

func buildHealthInterpretationResult(state domain.HealthTreatmentState) InterpretationResult {
	attributes := cloneAttributes(state.Attributes)
	if attributes == nil {
		attributes = make(map[string]string)
	}
	if strings.TrimSpace(state.DiagnosisDateText) != "" {
		attributes["diagnosis_date"] = strings.TrimSpace(state.DiagnosisDateText)
	}
	if strings.TrimSpace(state.Medicine) != "" {
		attributes["medicine"] = strings.TrimSpace(state.Medicine)
	}
	if state.TreatmentDays > 0 {
		attributes["treatment_days"] = strconv.Itoa(state.TreatmentDays)
	}

	return InterpretationResult{
		NormalizedIntent:     state.Category + "." + state.Subcategory,
		Category:             state.Category,
		Subcategory:          state.Subcategory,
		Description:          state.Description,
		AnimalCode:           state.AnimalCode,
		Confidence:           0.96,
		RequiresConfirmation: true,
		OccurredAt:           state.DiagnosisOccurredAt,
		Attributes:           attributes,
	}
}

func buildHealthTreatmentSourceText(state domain.HealthTreatmentState) string {
	parts := []string{strings.TrimSpace(state.Description)}
	if strings.TrimSpace(state.DiagnosisDateText) != "" {
		parts = append(parts, "diagnostico: "+strings.TrimSpace(state.DiagnosisDateText))
	}
	if strings.TrimSpace(state.Medicine) != "" {
		parts = append(parts, "medicamento: "+strings.TrimSpace(state.Medicine))
	}
	if state.TreatmentDays > 0 {
		parts = append(parts, "dias de tratamento: "+strconv.Itoa(state.TreatmentDays))
	}
	return strings.Join(parts, "; ")
}

func normalizeHealthAnswer(text string) string {
	return strings.TrimSpace(text)
}

func parseDiagnosisDate(text string) (time.Time, bool) {
	normalized := normalizeText(text)
	now := time.Now().UTC()
	if strings.Contains(normalized, "hoje") {
		return now, true
	}

	location := time.FixedZone("BRT", -3*60*60)
	for _, layout := range []string{"02/01/2006", "2/1/2006", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, strings.TrimSpace(text), location); err == nil {
			return parsed.UTC(), true
		}
	}

	return time.Time{}, false
}

func parseTreatmentDays(text string) (int, bool) {
	text = strings.TrimSpace(text)
	digits := make([]rune, 0, len(text))
	for _, r := range text {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) == 0 {
		return 0, false
	}
	value, err := strconv.Atoi(string(digits))
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func formatHealthDiagnosisDate(value time.Time) string {
	return value.In(time.FixedZone("BRT", -3*60*60)).Format("02/01/2006")
}

func cloneAttributes(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}
