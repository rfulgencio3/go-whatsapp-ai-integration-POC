package agro

import (
	"context"
	"strings"
	"testing"
	"time"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type stubMessageProcessor struct {
	result chatbot.ProcessResult
	err    error
	calls  int
}

func (s stubMessageProcessor) ProcessIncomingMessage(_ context.Context, _ chat.IncomingMessage) (chatbot.ProcessResult, error) {
	s.calls++
	return s.result, s.err
}

type countingMessageProcessor struct {
	result chatbot.ProcessResult
	err    error
	calls  int
}

func (s *countingMessageProcessor) ProcessIncomingMessage(_ context.Context, _ chat.IncomingMessage) (chatbot.ProcessResult, error) {
	s.calls++
	return s.result, s.err
}

type stubFarmMembershipRepository struct {
	memberships []domain.FarmMembership
	err         error
}

func (s stubFarmMembershipRepository) FindActiveByPhoneNumber(_ context.Context, _ string) ([]domain.FarmMembership, error) {
	return s.memberships, s.err
}

type stubFarmRegistrationRepository struct {
	membership domain.FarmMembership
	calls      int
	err        error
}

func (s *stubFarmRegistrationRepository) CreateInitialRegistration(_ context.Context, phoneNumber, producerName, farmName string) (domain.FarmMembership, error) {
	if s.err != nil {
		return domain.FarmMembership{}, s.err
	}
	s.calls++
	if s.membership.ID == "" {
		now := time.Now().UTC()
		s.membership = domain.FarmMembership{
			ID:          "membership-created",
			FarmID:      "farm-created",
			FarmName:    farmName,
			PersonName:  producerName,
			PhoneNumber: phoneNumber,
			Role:        domain.RoleOwner,
			IsPrimary:   true,
			Status:      "active",
			VerifiedAt:  &now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
	}
	return s.membership, nil
}

type stubPhoneContextStateRepository struct {
	states map[string]domain.PhoneContextState
	err    error
}

func (s *stubPhoneContextStateRepository) GetByPhoneNumber(_ context.Context, phoneNumber string) (domain.PhoneContextState, bool, error) {
	if s.err != nil {
		return domain.PhoneContextState{}, false, s.err
	}
	if s.states == nil {
		return domain.PhoneContextState{}, false, nil
	}
	state, ok := s.states[domain.NormalizePhoneNumber(phoneNumber)]
	return state, ok, nil
}

func (s *stubPhoneContextStateRepository) Upsert(_ context.Context, state *domain.PhoneContextState) error {
	if s.err != nil {
		return s.err
	}
	if s.states == nil {
		s.states = make(map[string]domain.PhoneContextState)
	}
	s.states[domain.NormalizePhoneNumber(state.PhoneNumber)] = *state
	return nil
}

type stubOnboardingStateRepository struct {
	states map[string]domain.OnboardingState
	err    error
}

func (s *stubOnboardingStateRepository) GetByPhoneNumber(_ context.Context, phoneNumber string) (domain.OnboardingState, bool, error) {
	if s.err != nil {
		return domain.OnboardingState{}, false, s.err
	}
	if s.states == nil {
		return domain.OnboardingState{}, false, nil
	}
	state, ok := s.states[domain.NormalizePhoneNumber(phoneNumber)]
	return state, ok, nil
}

func (s *stubOnboardingStateRepository) Upsert(_ context.Context, state *domain.OnboardingState) error {
	if s.err != nil {
		return s.err
	}
	if s.states == nil {
		s.states = make(map[string]domain.OnboardingState)
	}
	s.states[domain.NormalizePhoneNumber(state.PhoneNumber)] = *state
	return nil
}

func (s *stubOnboardingStateRepository) DeleteByPhoneNumber(_ context.Context, phoneNumber string) error {
	if s.err != nil {
		return s.err
	}
	if s.states != nil {
		delete(s.states, domain.NormalizePhoneNumber(phoneNumber))
	}
	return nil
}

type stubHealthTreatmentStateRepository struct {
	states map[string]domain.HealthTreatmentState
	err    error
}

func (s *stubHealthTreatmentStateRepository) GetByPhoneNumber(_ context.Context, phoneNumber string) (domain.HealthTreatmentState, bool, error) {
	if s.err != nil {
		return domain.HealthTreatmentState{}, false, s.err
	}
	if s.states == nil {
		return domain.HealthTreatmentState{}, false, nil
	}
	state, ok := s.states[domain.NormalizePhoneNumber(phoneNumber)]
	return state, ok, nil
}

func (s *stubHealthTreatmentStateRepository) Upsert(_ context.Context, state *domain.HealthTreatmentState) error {
	if s.err != nil {
		return s.err
	}
	if s.states == nil {
		s.states = make(map[string]domain.HealthTreatmentState)
	}
	s.states[domain.NormalizePhoneNumber(state.PhoneNumber)] = *state
	return nil
}

func (s *stubHealthTreatmentStateRepository) DeleteByPhoneNumber(_ context.Context, phoneNumber string) error {
	if s.err != nil {
		return s.err
	}
	if s.states != nil {
		delete(s.states, domain.NormalizePhoneNumber(phoneNumber))
	}
	return nil
}

type stubCorrelatedExpenseStateRepository struct {
	states map[string]domain.CorrelatedExpenseState
	err    error
}

func (s *stubCorrelatedExpenseStateRepository) GetByPhoneNumber(_ context.Context, phoneNumber string) (domain.CorrelatedExpenseState, bool, error) {
	if s.err != nil {
		return domain.CorrelatedExpenseState{}, false, s.err
	}
	if s.states == nil {
		return domain.CorrelatedExpenseState{}, false, nil
	}
	state, ok := s.states[domain.NormalizePhoneNumber(phoneNumber)]
	return state, ok, nil
}

func (s *stubCorrelatedExpenseStateRepository) Upsert(_ context.Context, state *domain.CorrelatedExpenseState) error {
	if s.err != nil {
		return s.err
	}
	if s.states == nil {
		s.states = make(map[string]domain.CorrelatedExpenseState)
	}
	s.states[domain.NormalizePhoneNumber(state.PhoneNumber)] = *state
	return nil
}

func (s *stubCorrelatedExpenseStateRepository) DeleteByPhoneNumber(_ context.Context, phoneNumber string) error {
	if s.err != nil {
		return s.err
	}
	if s.states != nil {
		delete(s.states, domain.NormalizePhoneNumber(phoneNumber))
	}
	return nil
}

type stubOnboardingMessageRepository struct {
	messages []domain.OnboardingMessage
	err      error
}

func (s *stubOnboardingMessageRepository) Create(_ context.Context, message *domain.OnboardingMessage) error {
	if s.err != nil {
		return s.err
	}
	s.messages = append(s.messages, *message)
	return nil
}

type stubConversationRepository struct {
	conversation domain.Conversation
	err          error
	calls        int
}

func (s *stubConversationRepository) GetOrCreateOpen(_ context.Context, _, _, _ string, _ time.Time) (domain.Conversation, error) {
	s.calls++
	return s.conversation, s.err
}

func (s *stubConversationRepository) SetPendingConfirmationEvent(_ context.Context, conversationID, eventID string) error {
	if s.conversation.ID == conversationID {
		s.conversation.PendingConfirmationEventID = eventID
	}
	return s.err
}

func (s *stubConversationRepository) SetPendingCorrectionEvent(_ context.Context, conversationID, eventID string) error {
	if s.conversation.ID == conversationID {
		s.conversation.PendingCorrectionEventID = eventID
	}
	return s.err
}

type stubSourceMessageRepository struct {
	messages []domain.SourceMessage
	err      error
}

func (s *stubSourceMessageRepository) Create(_ context.Context, message *domain.SourceMessage) error {
	if s.err != nil {
		return s.err
	}
	s.messages = append(s.messages, *message)
	return nil
}

type stubTranscriptionRepository struct {
	transcriptions []domain.Transcription
	err            error
}

func (s *stubTranscriptionRepository) Create(_ context.Context, transcription *domain.Transcription) error {
	if s.err != nil {
		return s.err
	}
	s.transcriptions = append(s.transcriptions, *transcription)
	return nil
}

type stubAssistantMessageRepository struct {
	messages []domain.AssistantMessage
	err      error
}

func (s *stubAssistantMessageRepository) Create(_ context.Context, message *domain.AssistantMessage) error {
	if s.err != nil {
		return s.err
	}
	s.messages = append(s.messages, *message)
	return nil
}

type stubInterpretationRunRepository struct {
	runs []domain.InterpretationRun
	err  error
}

func (s *stubInterpretationRunRepository) Create(_ context.Context, run *domain.InterpretationRun) error {
	if s.err != nil {
		return s.err
	}
	s.runs = append(s.runs, *run)
	return nil
}

type stubBusinessEventRepository struct {
	events          []domain.BusinessEvent
	correctionLinks map[string]string
	attributes      map[string]map[string]string
	milkWithdrawal  []domain.MilkWithdrawalAnimal
	recentHealth    []domain.HealthTreatmentSummary
	medicineMonth   float64
	vetMonth        float64
	recentPurchases []domain.InputPurchaseSummary
	err             error
}

func (s *stubBusinessEventRepository) Create(_ context.Context, event *domain.BusinessEvent) error {
	if s.err != nil {
		return s.err
	}
	s.events = append(s.events, *event)
	return nil
}

func (s *stubBusinessEventRepository) FindByID(_ context.Context, eventID string) (domain.BusinessEvent, bool, error) {
	if s.err != nil {
		return domain.BusinessEvent{}, false, s.err
	}
	for i := range s.events {
		event := s.events[i]
		if event.ID == eventID {
			return event, true, nil
		}
	}
	return domain.BusinessEvent{}, false, nil
}

func (s *stubBusinessEventRepository) ListActiveMilkWithdrawalAnimals(_ context.Context, _ string, _ time.Time) ([]domain.MilkWithdrawalAnimal, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]domain.MilkWithdrawalAnimal(nil), s.milkWithdrawal...), nil
}

func (s *stubBusinessEventRepository) ListRecentHealthTreatments(_ context.Context, _ string, _ int) ([]domain.HealthTreatmentSummary, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]domain.HealthTreatmentSummary(nil), s.recentHealth...), nil
}

func (s *stubBusinessEventRepository) SumMedicineExpensesForMonth(_ context.Context, _ string, _, _ time.Time) (float64, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.medicineMonth, nil
}

func (s *stubBusinessEventRepository) SumVetExpensesForMonth(_ context.Context, _ string, _, _ time.Time) (float64, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.vetMonth, nil
}

func (s *stubBusinessEventRepository) ListRecentInputPurchases(_ context.Context, _ string, _ int) ([]domain.InputPurchaseSummary, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]domain.InputPurchaseSummary(nil), s.recentPurchases...), nil
}

func (s *stubBusinessEventRepository) CreateCorrectionLink(_ context.Context, eventID, correctedEventID string) error {
	if s.err != nil {
		return s.err
	}
	if s.correctionLinks == nil {
		s.correctionLinks = make(map[string]string)
	}
	s.correctionLinks[eventID] = correctedEventID
	return nil
}

func (s *stubBusinessEventRepository) CreateAttributes(_ context.Context, eventID string, attributes map[string]string) error {
	if s.err != nil {
		return s.err
	}
	if len(attributes) == 0 {
		return nil
	}
	if s.attributes == nil {
		s.attributes = make(map[string]map[string]string)
	}
	copied := make(map[string]string, len(attributes))
	for key, value := range attributes {
		copied[key] = value
	}
	s.attributes[eventID] = copied
	return nil
}

func (s *stubBusinessEventRepository) UpdateStatus(_ context.Context, eventID string, status domain.EventStatus, confirmedByUser bool, confirmedAt *time.Time) error {
	if s.err != nil {
		return s.err
	}
	for i := range s.events {
		if s.events[i].ID != eventID {
			continue
		}
		s.events[i].Status = status
		s.events[i].ConfirmedByUser = confirmedByUser
		s.events[i].ConfirmedAt = confirmedAt
		return nil
	}
	return nil
}

type stubChatMessageSender struct {
	lastPhone string
	lastBody  string
	sendCount int
	err       error
}

func (s *stubChatMessageSender) SendTextMessage(_ context.Context, phoneNumber, body string) error {
	if s.err != nil {
		return s.err
	}
	s.lastPhone = phoneNumber
	s.lastBody = body
	s.sendCount++
	return nil
}

type stubChatConversationRepository struct {
	messages map[string][]chat.Message
}

func newStubChatConversationRepository() *stubChatConversationRepository {
	return &stubChatConversationRepository{messages: make(map[string][]chat.Message)}
}

func (s *stubChatConversationRepository) GetMessages(_ context.Context, phoneNumber string) ([]chat.Message, error) {
	return append([]chat.Message(nil), s.messages[phoneNumber]...), nil
}

func (s *stubChatConversationRepository) AppendMessage(_ context.Context, phoneNumber string, message chat.Message) error {
	s.messages[phoneNumber] = append(s.messages[phoneNumber], message)
	return nil
}

type stubChatMessageArchive struct {
	recorded []chat.Message
	err      error
}

func (s *stubChatMessageArchive) RecordMessage(_ context.Context, _ string, message chat.Message) error {
	if s.err != nil {
		return s.err
	}
	s.recorded = append(s.recorded, message)
	return nil
}

func TestCaptureServicePersistsSourceMessageWhenContextIsResolved(t *testing.T) {
	t.Parallel()

	conversations := &stubConversationRepository{
		conversation: domain.Conversation{
			ID:     "conv-1",
			FarmID: "farm-1",
		},
	}
	sourceMessages := &stubSourceMessageRepository{}
	transcriptions := &stubTranscriptionRepository{}
	interpretationRuns := &stubInterpretationRunRepository{}
	businessEvents := &stubBusinessEventRepository{}
	assistantMessages := &stubAssistantMessageRepository{}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	service := NewCaptureService(
		nil,
		stubMessageProcessor{result: chatbot.ProcessResult{
			PhoneNumber: "5511999999999",
			IncomingMessage: chat.IncomingMessage{
				MessageID:   "msg-1",
				PhoneNumber: "5511999999999",
				Text:        "Comprei 10 sacos de racao por 850 reais",
				Type:        chat.MessageTypeText,
				Provider:    "whatsmeow",
			},
			AssistantMessage: chat.Message{
				Role:     chat.AssistantRole,
				Text:     "Registrei a sua mensagem.",
				Provider: "whatsmeow",
			},
		}},
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5511999999999", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		conversations,
		sourceMessages,
		transcriptions,
		interpretationRuns,
		businessEvents,
		assistantMessages,
	)

	_, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "msg-1",
		PhoneNumber: "+55 (11) 99999-9999",
		Text:        "Comprei 10 sacos de racao por 850 reais",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if conversations.calls != 1 {
		t.Fatalf("expected conversation lookup once, got %d", conversations.calls)
	}
	if len(sourceMessages.messages) != 1 {
		t.Fatalf("expected one source message, got %d", len(sourceMessages.messages))
	}
	if got := sourceMessages.messages[0].ConversationID; got != "conv-1" {
		t.Fatalf("expected conversation id conv-1, got %q", got)
	}
	if got := sourceMessages.messages[0].SenderPhoneNumber; got != "5511999999999" {
		t.Fatalf("expected normalized phone number, got %q", got)
	}
	if len(transcriptions.transcriptions) != 0 {
		t.Fatalf("expected no transcription for plain text, got %d", len(transcriptions.transcriptions))
	}
	if sender.sendCount != 1 {
		t.Fatalf("expected one confirmation prompt to be sent, got %d", sender.sendCount)
	}
	if len(interpretationRuns.runs) != 1 {
		t.Fatalf("expected one interpretation run, got %d", len(interpretationRuns.runs))
	}
	if got := interpretationRuns.runs[0].NormalizedIntent; got != "finance.input_purchase" {
		t.Fatalf("expected finance.input_purchase, got %q", got)
	}
	if len(businessEvents.events) != 1 {
		t.Fatalf("expected one business event, got %d", len(businessEvents.events))
	}
	if got := businessEvents.events[0].Subcategory; got != "input_purchase" {
		t.Fatalf("expected input_purchase event, got %q", got)
	}
	if len(assistantMessages.messages) != 2 {
		t.Fatalf("expected two assistant messages, got %d", len(assistantMessages.messages))
	}
	if conversations.conversation.PendingConfirmationEventID == "" {
		t.Fatalf("expected pending confirmation event id to be set")
	}
	if assistantMessages.messages[1].ReplyType != domain.ReplyTypeConfirmation {
		t.Fatalf("expected second assistant message to be a confirmation prompt, got %q", assistantMessages.messages[1].ReplyType)
	}
	if got := assistantMessages.messages[0].SourceMessageID; got != sourceMessages.messages[0].ID {
		t.Fatalf("expected assistant message to link source message, got %q", got)
	}
}

func TestCaptureServiceSkipsPersistenceWhenMessageIsDuplicate(t *testing.T) {
	t.Parallel()

	conversations := &stubConversationRepository{}
	sourceMessages := &stubSourceMessageRepository{}
	transcriptions := &stubTranscriptionRepository{}
	interpretationRuns := &stubInterpretationRunRepository{}
	businessEvents := &stubBusinessEventRepository{}
	assistantMessages := &stubAssistantMessageRepository{}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	service := NewCaptureService(
		nil,
		stubMessageProcessor{result: chatbot.ProcessResult{Duplicate: true}},
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5511999999999", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		conversations,
		sourceMessages,
		transcriptions,
		interpretationRuns,
		businessEvents,
		assistantMessages,
	)

	_, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "msg-1",
		PhoneNumber: "5511999999999",
		Text:        "teste",
		Type:        chat.MessageTypeText,
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if conversations.calls != 0 {
		t.Fatalf("expected no conversation lookup for duplicate, got %d", conversations.calls)
	}
	if len(sourceMessages.messages) != 0 {
		t.Fatalf("expected no source messages for duplicate, got %d", len(sourceMessages.messages))
	}
	if len(transcriptions.transcriptions) != 0 {
		t.Fatalf("expected no transcriptions for duplicate, got %d", len(transcriptions.transcriptions))
	}
	if len(assistantMessages.messages) != 0 {
		t.Fatalf("expected no assistant messages for duplicate, got %d", len(assistantMessages.messages))
	}
	if len(interpretationRuns.runs) != 0 {
		t.Fatalf("expected no interpretation runs for duplicate, got %d", len(interpretationRuns.runs))
	}
	if len(businessEvents.events) != 0 {
		t.Fatalf("expected no business events for duplicate, got %d", len(businessEvents.events))
	}
}

func TestCaptureServiceRepliesWhenPhoneNumberIsNotRegistered(t *testing.T) {
	t.Parallel()

	processor := &countingMessageProcessor{}
	sourceMessages := &stubSourceMessageRepository{}
	transcriptions := &stubTranscriptionRepository{}
	interpretationRuns := &stubInterpretationRunRepository{}
	businessEvents := &stubBusinessEventRepository{}
	assistantMessages := &stubAssistantMessageRepository{}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	service := NewCaptureService(
		nil,
		processor,
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{},
		nil,
		nil,
		nil,
		&stubConversationRepository{},
		sourceMessages,
		transcriptions,
		interpretationRuns,
		businessEvents,
		assistantMessages,
	)

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "msg-unregistered",
		PhoneNumber: "5511999999999",
		Text:        "Quero registrar uma compra",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if processor.calls != 0 {
		t.Fatalf("expected downstream not to be called, got %d", processor.calls)
	}
	if sender.sendCount != 1 {
		t.Fatalf("expected one onboarding reply, got %d", sender.sendCount)
	}
	if got := result.AssistantMessage.Text; got != buildUnregisteredNumberReply() {
		t.Fatalf("expected unregistered reply, got %q", got)
	}
	if len(chatHistory.messages["5511999999999"]) != 2 {
		t.Fatalf("expected legacy conversation to store user and assistant messages")
	}
	if len(archive.recorded) != 2 {
		t.Fatalf("expected legacy archive to store onboarding flow")
	}
	if len(sourceMessages.messages) != 0 || len(transcriptions.transcriptions) != 0 || len(interpretationRuns.runs) != 0 || len(businessEvents.events) != 0 || len(assistantMessages.messages) != 0 {
		t.Fatalf("expected no agro persistence for unregistered number")
	}
}

func TestCaptureServiceStartsOnboardingForUnregisteredPhoneNumber(t *testing.T) {
	t.Parallel()

	processor := &countingMessageProcessor{}
	onboardingStates := &stubOnboardingStateRepository{}
	onboardingMessages := &stubOnboardingMessageRepository{}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	service := NewCaptureService(
		nil,
		processor,
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{},
		nil,
		nil,
		onboardingStates,
		&stubConversationRepository{},
		&stubSourceMessageRepository{},
		&stubTranscriptionRepository{},
		&stubInterpretationRunRepository{},
		&stubBusinessEventRepository{},
		&stubAssistantMessageRepository{},
		onboardingMessages,
	)

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "msg-onboarding-start",
		PhoneNumber: "5511999999999",
		Text:        "cadastrar",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if processor.calls != 0 {
		t.Fatalf("expected downstream not to be called, got %d", processor.calls)
	}
	if got := result.AssistantMessage.Text; got != "Vamos fazer seu cadastro inicial. Qual o nome do produtor ou responsavel?" {
		t.Fatalf("expected onboarding start reply, got %q", got)
	}
	state, found, err := onboardingStates.GetByPhoneNumber(context.Background(), "5511999999999")
	if err != nil {
		t.Fatalf("GetByPhoneNumber() error = %v", err)
	}
	if !found || state.Step != domain.OnboardingStepAwaitingProducerName {
		t.Fatalf("expected onboarding state awaiting producer name, got %+v", state)
	}
	if len(onboardingMessages.messages) != 2 {
		t.Fatalf("expected onboarding messages to be persisted, got %d", len(onboardingMessages.messages))
	}
	if onboardingMessages.messages[0].Direction != domain.OnboardingMessageDirectionInbound {
		t.Fatalf("expected first onboarding message to be inbound, got %q", onboardingMessages.messages[0].Direction)
	}
	if onboardingMessages.messages[1].Direction != domain.OnboardingMessageDirectionOutbound {
		t.Fatalf("expected second onboarding message to be outbound, got %q", onboardingMessages.messages[1].Direction)
	}
	if onboardingMessages.messages[1].Step != domain.OnboardingStepAwaitingProducerName {
		t.Fatalf("expected onboarding step awaiting producer name, got %q", onboardingMessages.messages[1].Step)
	}
}

func TestCaptureServiceCompletesOnboardingFlow(t *testing.T) {
	t.Parallel()

	processor := &countingMessageProcessor{}
	registration := &stubFarmRegistrationRepository{}
	onboardingStates := &stubOnboardingStateRepository{
		states: map[string]domain.OnboardingState{
			"5511999999999": {
				PhoneNumber: "5511999999999",
				Step:        domain.OnboardingStepAwaitingProducerName,
			},
		},
	}
	onboardingMessages := &stubOnboardingMessageRepository{}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	service := NewCaptureService(
		nil,
		processor,
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{},
		registration,
		nil,
		onboardingStates,
		&stubConversationRepository{},
		&stubSourceMessageRepository{},
		&stubTranscriptionRepository{},
		&stubInterpretationRunRepository{},
		&stubBusinessEventRepository{},
		&stubAssistantMessageRepository{},
		onboardingMessages,
	)

	producerStep, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "msg-onboarding-producer",
		PhoneNumber: "5511999999999",
		Text:        "Joao da Silva",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("producer step ProcessIncomingMessage() error = %v", err)
	}
	if got := producerStep.AssistantMessage.Text; got != "Agora envie o nome da fazenda ou negocio." {
		t.Fatalf("expected onboarding producer reply, got %q", got)
	}

	state, found, err := onboardingStates.GetByPhoneNumber(context.Background(), "5511999999999")
	if err != nil {
		t.Fatalf("GetByPhoneNumber() error = %v", err)
	}
	if !found || state.Step != domain.OnboardingStepAwaitingFarmName || state.ProducerName != "Joao da Silva" {
		t.Fatalf("expected onboarding state awaiting farm name, got %+v", state)
	}

	farmStep, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "msg-onboarding-farm",
		PhoneNumber: "5511999999999",
		Text:        "Fazenda Boa Vista",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("farm step ProcessIncomingMessage() error = %v", err)
	}
	if registration.calls != 1 {
		t.Fatalf("expected registration to be created once, got %d", registration.calls)
	}
	if got := farmStep.AssistantMessage.Text; got != "Cadastro concluido. Seu numero foi vinculado a Fazenda Boa Vista. Agora voce ja pode enviar registros." {
		t.Fatalf("expected onboarding completion reply, got %q", got)
	}
	if _, found, err := onboardingStates.GetByPhoneNumber(context.Background(), "5511999999999"); err != nil || found {
		t.Fatalf("expected onboarding state to be deleted, found=%v err=%v", found, err)
	}
	if len(onboardingMessages.messages) != 4 {
		t.Fatalf("expected onboarding trace for both steps, got %d", len(onboardingMessages.messages))
	}
	lastMessage := onboardingMessages.messages[len(onboardingMessages.messages)-1]
	if lastMessage.Direction != domain.OnboardingMessageDirectionOutbound {
		t.Fatalf("expected last onboarding message to be outbound, got %q", lastMessage.Direction)
	}
	if lastMessage.Body != "Cadastro concluido. Seu numero foi vinculado a Fazenda Boa Vista. Agora voce ja pode enviar registros." {
		t.Fatalf("unexpected onboarding completion trace: %q", lastMessage.Body)
	}
}

func TestCaptureServiceRepliesWhenPhoneNumberHasAmbiguousContext(t *testing.T) {
	t.Parallel()

	processor := &countingMessageProcessor{}
	phoneContexts := &stubPhoneContextStateRepository{}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	service := NewCaptureService(
		nil,
		processor,
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", FarmName: "Fazenda Boa Vista", PhoneNumber: "5511999999999", Status: "active"},
				{ID: "membership-2", FarmID: "farm-2", FarmName: "Sitio Santa Luzia", PhoneNumber: "5511999999999", Status: "active"},
			},
		},
		nil,
		phoneContexts,
		nil,
		&stubConversationRepository{},
		&stubSourceMessageRepository{},
		&stubTranscriptionRepository{},
		&stubInterpretationRunRepository{},
		&stubBusinessEventRepository{},
		&stubAssistantMessageRepository{},
	)

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "msg-ambiguous",
		PhoneNumber: "5511999999999",
		Text:        "Quero registrar uma compra",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if processor.calls != 0 {
		t.Fatalf("expected downstream not to be called, got %d", processor.calls)
	}
	if sender.sendCount != 1 {
		t.Fatalf("expected one ambiguous-context reply, got %d", sender.sendCount)
	}
	expectedReply := "Seu numero esta vinculado a mais de uma fazenda. Responda com o numero:\n1. Fazenda Boa Vista\n2. Sitio Santa Luzia"
	if got := result.AssistantMessage.Text; got != expectedReply {
		t.Fatalf("expected ambiguous-context selection reply, got %q", got)
	}
	if len(chatHistory.messages["5511999999999"]) != 2 {
		t.Fatalf("expected legacy conversation to store user and assistant messages")
	}
	if len(archive.recorded) != 2 {
		t.Fatalf("expected legacy archive to store ambiguous flow")
	}
	state, found, err := phoneContexts.GetByPhoneNumber(context.Background(), "5511999999999")
	if err != nil {
		t.Fatalf("GetByPhoneNumber() error = %v", err)
	}
	if !found || len(state.PendingOptions) != 2 {
		t.Fatalf("expected pending context options to be persisted")
	}
}

func TestCaptureServiceSelectsContextForAmbiguousPhoneNumber(t *testing.T) {
	t.Parallel()

	processor := &countingMessageProcessor{}
	phoneContexts := &stubPhoneContextStateRepository{
		states: map[string]domain.PhoneContextState{
			"5511999999999": {
				PhoneNumber: "5511999999999",
				PendingOptions: []domain.PhoneContextOption{
					{FarmID: "farm-1", FarmName: "Fazenda Boa Vista"},
					{FarmID: "farm-2", FarmName: "Sitio Santa Luzia"},
				},
			},
		},
	}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	service := NewCaptureService(
		nil,
		processor,
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", FarmName: "Fazenda Boa Vista", PhoneNumber: "5511999999999", Status: "active"},
				{ID: "membership-2", FarmID: "farm-2", FarmName: "Sitio Santa Luzia", PhoneNumber: "5511999999999", Status: "active"},
			},
		},
		nil,
		phoneContexts,
		nil,
		&stubConversationRepository{},
		&stubSourceMessageRepository{},
		&stubTranscriptionRepository{},
		&stubInterpretationRunRepository{},
		&stubBusinessEventRepository{},
		&stubAssistantMessageRepository{},
	)

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "msg-select",
		PhoneNumber: "5511999999999",
		Text:        "2",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if processor.calls != 0 {
		t.Fatalf("expected downstream not to be called, got %d", processor.calls)
	}
	if got := result.AssistantMessage.Text; got != "Contexto definido para Sitio Santa Luzia. Envie a informacao novamente." {
		t.Fatalf("expected selected-context reply, got %q", got)
	}
	state, found, err := phoneContexts.GetByPhoneNumber(context.Background(), "5511999999999")
	if err != nil {
		t.Fatalf("GetByPhoneNumber() error = %v", err)
	}
	if !found || state.ActiveFarmID != "farm-2" {
		t.Fatalf("expected active farm to be farm-2, got %+v", state)
	}
	if len(state.PendingOptions) != 0 {
		t.Fatalf("expected pending options to be cleared")
	}
}

func TestCaptureServiceSwitchesContextWhenRequested(t *testing.T) {
	t.Parallel()

	processor := &countingMessageProcessor{}
	phoneContexts := &stubPhoneContextStateRepository{
		states: map[string]domain.PhoneContextState{
			"5511999999999": {
				PhoneNumber:    "5511999999999",
				ActiveFarmID:   "farm-1",
				PendingOptions: nil,
			},
		},
	}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	service := NewCaptureService(
		nil,
		processor,
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", FarmName: "Fazenda Boa Vista", PhoneNumber: "5511999999999", Status: "active"},
				{ID: "membership-2", FarmID: "farm-2", FarmName: "Sitio Santa Luzia", PhoneNumber: "5511999999999", Status: "active"},
			},
		},
		nil,
		phoneContexts,
		nil,
		&stubConversationRepository{},
		&stubSourceMessageRepository{},
		&stubTranscriptionRepository{},
		&stubInterpretationRunRepository{},
		&stubBusinessEventRepository{},
		&stubAssistantMessageRepository{},
	)

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "msg-switch",
		PhoneNumber: "5511999999999",
		Text:        "trocar fazenda",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if processor.calls != 0 {
		t.Fatalf("expected downstream not to be called, got %d", processor.calls)
	}
	expectedReply := "Seu numero esta vinculado a mais de uma fazenda. Responda com o numero:\n1. Fazenda Boa Vista\n2. Sitio Santa Luzia"
	if got := result.AssistantMessage.Text; got != expectedReply {
		t.Fatalf("expected switch-context selection reply, got %q", got)
	}
	state, found, err := phoneContexts.GetByPhoneNumber(context.Background(), "5511999999999")
	if err != nil {
		t.Fatalf("GetByPhoneNumber() error = %v", err)
	}
	if !found || state.ActiveFarmID != "" {
		t.Fatalf("expected active farm to be cleared, got %+v", state)
	}
	if len(state.PendingOptions) != 2 {
		t.Fatalf("expected pending options after switch request")
	}
}

func TestCaptureServiceRepliesWhenSwitchRequestedWithSingleContext(t *testing.T) {
	t.Parallel()

	processor := &countingMessageProcessor{}
	phoneContexts := &stubPhoneContextStateRepository{}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	service := NewCaptureService(
		nil,
		processor,
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", FarmName: "Fazenda Boa Vista", PhoneNumber: "5511999999999", Status: "active"},
			},
		},
		nil,
		phoneContexts,
		nil,
		&stubConversationRepository{},
		&stubSourceMessageRepository{},
		&stubTranscriptionRepository{},
		&stubInterpretationRunRepository{},
		&stubBusinessEventRepository{},
		&stubAssistantMessageRepository{},
	)

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "msg-switch-single",
		PhoneNumber: "5511999999999",
		Text:        "trocar fazenda",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if processor.calls != 0 {
		t.Fatalf("expected downstream not to be called, got %d", processor.calls)
	}
	if got := result.AssistantMessage.Text; got != "Seu numero ja esta vinculado a Fazenda Boa Vista." {
		t.Fatalf("expected single-context reply, got %q", got)
	}
}

func TestCaptureServicePersistsTranscriptionForAudioMessages(t *testing.T) {
	t.Parallel()

	conversations := &stubConversationRepository{
		conversation: domain.Conversation{
			ID:     "conv-1",
			FarmID: "farm-1",
		},
	}
	sourceMessages := &stubSourceMessageRepository{}
	transcriptions := &stubTranscriptionRepository{}
	interpretationRuns := &stubInterpretationRunRepository{}
	businessEvents := &stubBusinessEventRepository{}
	assistantMessages := &stubAssistantMessageRepository{}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	service := NewCaptureService(
		nil,
		stubMessageProcessor{result: chatbot.ProcessResult{
			PhoneNumber: "5511999999999",
			IncomingMessage: chat.IncomingMessage{
				MessageID:             "audio-1",
				PhoneNumber:           "5511999999999",
				Text:                  "a vaca 32 foi inseminada hoje",
				Type:                  chat.MessageTypeAudio,
				Provider:              "twilio",
				TranscriptionID:       "external-transcription-id",
				TranscriptionLanguage: "pt-BR",
				AudioDurationSeconds:  11.2,
			},
			AssistantMessage: chat.Message{
				Role:     chat.AssistantRole,
				Text:     "Entendi. Vou registrar esse evento.",
				Provider: "twilio",
			},
		}},
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5511999999999", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		conversations,
		sourceMessages,
		transcriptions,
		interpretationRuns,
		businessEvents,
		assistantMessages,
	)

	_, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "audio-1",
		PhoneNumber: "5511999999999",
		Type:        chat.MessageTypeAudio,
		Provider:    "twilio",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if len(sourceMessages.messages) != 1 {
		t.Fatalf("expected one source message, got %d", len(sourceMessages.messages))
	}
	if len(transcriptions.transcriptions) != 1 {
		t.Fatalf("expected one transcription, got %d", len(transcriptions.transcriptions))
	}
	if sender.sendCount != 1 {
		t.Fatalf("expected one confirmation prompt to be sent, got %d", sender.sendCount)
	}
	if got := transcriptions.transcriptions[0].ProviderRef; got != "external-transcription-id" {
		t.Fatalf("expected transcription provider ref to be preserved, got %q", got)
	}
	if got := transcriptions.transcriptions[0].SourceMessageID; got != sourceMessages.messages[0].ID {
		t.Fatalf("expected transcription to link source message, got %q", got)
	}
	if len(interpretationRuns.runs) != 1 {
		t.Fatalf("expected one interpretation run, got %d", len(interpretationRuns.runs))
	}
	if got := interpretationRuns.runs[0].NormalizedIntent; got != "reproduction.insemination" {
		t.Fatalf("expected reproduction.insemination, got %q", got)
	}
	if len(businessEvents.events) != 1 {
		t.Fatalf("expected one business event, got %d", len(businessEvents.events))
	}
	if got := businessEvents.events[0].Subcategory; got != "insemination" {
		t.Fatalf("expected insemination event, got %q", got)
	}
	if len(assistantMessages.messages) != 2 {
		t.Fatalf("expected two assistant messages, got %d", len(assistantMessages.messages))
	}
}

func TestCaptureServiceConfirmsLatestDraftEvent(t *testing.T) {
	t.Parallel()

	conversations := &stubConversationRepository{
		conversation: domain.Conversation{
			ID:     "conv-1",
			FarmID: "farm-1",
		},
	}
	sourceMessages := &stubSourceMessageRepository{}
	transcriptions := &stubTranscriptionRepository{}
	interpretationRuns := &stubInterpretationRunRepository{}
	businessEvents := &stubBusinessEventRepository{
		events: []domain.BusinessEvent{
			{
				ID:          "event-1",
				FarmID:      "farm-1",
				Category:    "finance",
				Subcategory: "input_purchase",
				Amount:      float64Ptr(850),
				Quantity:    float64Ptr(10),
				Unit:        "saco",
				Status:      domain.EventStatusDraft,
			},
		},
	}
	conversations.conversation.PendingConfirmationEventID = "event-1"
	assistantMessages := &stubAssistantMessageRepository{}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	processor := stubMessageProcessor{}
	service := NewCaptureService(
		nil,
		processor,
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5511999999999", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		conversations,
		sourceMessages,
		transcriptions,
		interpretationRuns,
		businessEvents,
		assistantMessages,
	)

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "confirm-1",
		PhoneNumber: "5511999999999",
		Text:        "sim",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if result.AssistantMessage.Text == "" {
		t.Fatalf("expected assistant confirmation message")
	}
	if sender.sendCount != 1 {
		t.Fatalf("expected one outbound confirmation message, got %d", sender.sendCount)
	}
	if businessEvents.events[0].Status != domain.EventStatusConfirmed {
		t.Fatalf("expected event to be confirmed, got %q", businessEvents.events[0].Status)
	}
	if conversations.conversation.PendingConfirmationEventID != "" {
		t.Fatalf("expected pending confirmation event id to be cleared")
	}
	if conversations.conversation.PendingCorrectionEventID != "" {
		t.Fatalf("expected pending correction event id to stay empty")
	}
	if !businessEvents.events[0].ConfirmedByUser {
		t.Fatalf("expected event confirmed by user")
	}
	if len(sourceMessages.messages) != 1 {
		t.Fatalf("expected confirmation source message to be saved, got %d", len(sourceMessages.messages))
	}
	if len(assistantMessages.messages) != 1 {
		t.Fatalf("expected confirmation assistant message to be saved, got %d", len(assistantMessages.messages))
	}
	if assistantMessages.messages[0].ReplyType != domain.ReplyTypeConfirmation {
		t.Fatalf("expected reply type confirmation, got %q", assistantMessages.messages[0].ReplyType)
	}
	if len(chatHistory.messages["5511999999999"]) != 2 {
		t.Fatalf("expected confirmation flow persisted in legacy conversation history")
	}
	if len(archive.recorded) != 2 {
		t.Fatalf("expected confirmation flow archived in legacy archive")
	}
}

func TestCaptureServiceRejectsDraftAndLinksCorrectionMessage(t *testing.T) {
	t.Parallel()

	conversations := &stubConversationRepository{
		conversation: domain.Conversation{
			ID:                         "conv-1",
			FarmID:                     "farm-1",
			PendingConfirmationEventID: "event-1",
		},
	}
	sourceMessages := &stubSourceMessageRepository{}
	transcriptions := &stubTranscriptionRepository{}
	interpretationRuns := &stubInterpretationRunRepository{}
	businessEvents := &stubBusinessEventRepository{
		events: []domain.BusinessEvent{
			{
				ID:          "event-1",
				FarmID:      "farm-1",
				Category:    "finance",
				Subcategory: "input_purchase",
				Amount:      float64Ptr(850),
				Quantity:    float64Ptr(10),
				Unit:        "saco",
				Status:      domain.EventStatusDraft,
			},
		},
	}
	assistantMessages := &stubAssistantMessageRepository{}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	service := NewCaptureService(
		nil,
		stubMessageProcessor{result: chatbot.ProcessResult{
			PhoneNumber: "5511999999999",
			IncomingMessage: chat.IncomingMessage{
				MessageID:   "msg-2",
				PhoneNumber: "5511999999999",
				Text:        "Comprei 8 sacos de racao por 700 reais",
				Type:        chat.MessageTypeText,
				Provider:    "whatsmeow",
			},
			AssistantMessage: chat.Message{
				Role:     chat.AssistantRole,
				Text:     "Registrei compra de insumos de R$ 700.00, 8 saco. Responda SIM para confirmar ou NAO para corrigir.",
				Provider: "whatsmeow",
			},
			AssistantReplyKind: chatbot.ReplyKindConfirmation,
		}},
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5511999999999", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		conversations,
		sourceMessages,
		transcriptions,
		interpretationRuns,
		businessEvents,
		assistantMessages,
	)

	rejectResult, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "reject-1",
		PhoneNumber: "5511999999999",
		Text:        "nao",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("reject ProcessIncomingMessage() error = %v", err)
	}
	if rejectResult.AssistantMessage.Text == "" {
		t.Fatalf("expected reject assistant message")
	}
	if conversations.conversation.PendingConfirmationEventID != "" {
		t.Fatalf("expected pending confirmation event to be cleared after rejection")
	}
	if conversations.conversation.PendingCorrectionEventID != "event-1" {
		t.Fatalf("expected pending correction event to point to rejected event, got %q", conversations.conversation.PendingCorrectionEventID)
	}
	if businessEvents.events[0].Status != domain.EventStatusRejected {
		t.Fatalf("expected first event to be rejected, got %q", businessEvents.events[0].Status)
	}

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "msg-2",
		PhoneNumber: "5511999999999",
		Text:        "Comprei 8 sacos de racao por 700 reais",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("corrected ProcessIncomingMessage() error = %v", err)
	}

	if result.AssistantReplyKind != chatbot.ReplyKindConfirmation {
		t.Fatalf("expected corrected message to keep confirmation reply kind, got %q", result.AssistantReplyKind)
	}
	if len(businessEvents.events) != 2 {
		t.Fatalf("expected new corrected business event, got %d", len(businessEvents.events))
	}
	newEventID := businessEvents.events[1].ID
	if got := businessEvents.correctionLinks[newEventID]; got != "event-1" {
		t.Fatalf("expected corrected event to link to event-1, got %q", got)
	}
	if conversations.conversation.PendingCorrectionEventID != "" {
		t.Fatalf("expected pending correction event to be cleared after corrected message")
	}
	if conversations.conversation.PendingConfirmationEventID != newEventID {
		t.Fatalf("expected pending confirmation to point to new draft event, got %q", conversations.conversation.PendingConfirmationEventID)
	}
}

func TestBuildDraftConfirmationPromptFromInterpretation(t *testing.T) {
	t.Parallel()

	occurredAt := time.Date(2026, time.April, 7, 9, 30, 0, 0, time.UTC)
	prompt := buildDraftConfirmationPromptFromInterpretation(InterpretationResult{
		Category:    "finance",
		Subcategory: "input_purchase",
		Description: "Comprei 10 sacos de racao por 850 reais",
		Amount:      float64Ptr(850),
		Currency:    "BRL",
		Quantity:    float64Ptr(10),
		Unit:        "saco",
		OccurredAt:  &occurredAt,
	})

	expected := "Categoria: Compra de insumos\nDescricao: Comprei 10 sacos de racao por 850 reais\nValor: R$ 850.00\nQuantidade: 10 saco\nData: 07/04/2026 06:30\nResponda SIM para confirmar ou NAO para corrigir."
	if prompt != expected {
		t.Fatalf("unexpected confirmation prompt:\n%s", prompt)
	}
}

func TestBuildDraftConfirmationPromptFromInterpretationHealth(t *testing.T) {
	t.Parallel()

	prompt := buildDraftConfirmationPromptFromInterpretation(InterpretationResult{
		Category:    "health",
		Subcategory: "mastitis_treatment",
		Description: "A vaca 32 esta com problema nas tetas T1 e T3 e nao pode tirar leite",
		AnimalCode:  "32",
		Attributes: map[string]string{
			"affected_teats":  "T1,T3",
			"milk_withdrawal": "true",
		},
	})

	expected := "Categoria: Saude animal\nAnimal: 32\nProblema: teta/mastite\nTetas afetadas: T1,T3\nRestricao: nao tirar leite\nDescricao: A vaca 32 esta com problema nas tetas T1 e T3 e nao pode tirar leite\nResponda SIM para confirmar ou NAO para corrigir."
	if prompt != expected {
		t.Fatalf("unexpected health confirmation prompt:\n%s", prompt)
	}
}

func TestCaptureServiceStartsHealthTreatmentFlow(t *testing.T) {
	t.Parallel()

	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	processor := &countingMessageProcessor{}
	healthStates := &stubHealthTreatmentStateRepository{}

	service := NewCaptureService(
		nil,
		processor,
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "553488283531", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		&stubConversationRepository{},
		&stubSourceMessageRepository{},
		&stubTranscriptionRepository{},
		&stubInterpretationRunRepository{},
		&stubBusinessEventRepository{},
		&stubAssistantMessageRepository{},
	)
	service.SetHealthTreatmentStateRepository(healthStates)

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "msg-health-1",
		PhoneNumber: "5534988283531",
		Text:        "A vaca 32 esta com problema na teta T3 e nao pode tirar leite",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if processor.calls != 0 {
		t.Fatalf("expected downstream not to be called, got %d calls", processor.calls)
	}
	if sender.sendCount != 1 {
		t.Fatalf("expected one outbound question, got %d", sender.sendCount)
	}
	if sender.lastBody != "Qual a data do diagnostico?" {
		t.Fatalf("unexpected health question: %q", sender.lastBody)
	}

	state, found, _ := healthStates.GetByPhoneNumber(context.Background(), "5534988283531")
	if !found {
		t.Fatalf("expected health treatment state to be persisted")
	}
	if state.Step != domain.HealthTreatmentStepAwaitingDiagnosisDate {
		t.Fatalf("expected awaiting diagnosis date step, got %q", state.Step)
	}
	if state.AnimalCode != "32" {
		t.Fatalf("expected animal code 32, got %q", state.AnimalCode)
	}
	if got := result.AssistantMessage.Text; got != "Qual a data do diagnostico?" {
		t.Fatalf("unexpected assistant message %q", got)
	}
}

func TestCaptureServiceCompletesHealthTreatmentFlow(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}
	conversations := &stubConversationRepository{
		conversation: domain.Conversation{
			ID:     "conv-1",
			FarmID: "farm-1",
		},
	}
	sourceMessages := &stubSourceMessageRepository{}
	interpretationRuns := &stubInterpretationRunRepository{}
	businessEvents := &stubBusinessEventRepository{}
	assistantMessages := &stubAssistantMessageRepository{}
	healthStates := &stubHealthTreatmentStateRepository{
		states: map[string]domain.HealthTreatmentState{
			"5534988283531": {
				PhoneNumber: "5534988283531",
				FarmID:      "farm-1",
				Category:    "health",
				Subcategory: "mastitis_treatment",
				AnimalCode:  "32",
				Description: "A vaca 32 esta com problema na teta T3 e nao pode tirar leite",
				Attributes: map[string]string{
					"affected_teats":    "T3",
					"milk_withdrawal":   "true",
					"health_issue_type": "teta",
				},
				DiagnosisDateText:   "hoje",
				DiagnosisOccurredAt: &now,
				Medicine:            "Mastclin",
				Step:                domain.HealthTreatmentStepAwaitingTreatmentDays,
				UpdatedAt:           now,
			},
		},
	}

	service := NewCaptureService(
		nil,
		&countingMessageProcessor{},
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5534988283531", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		conversations,
		sourceMessages,
		&stubTranscriptionRepository{},
		interpretationRuns,
		businessEvents,
		assistantMessages,
	)
	service.SetHealthTreatmentStateRepository(healthStates)

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "msg-health-2",
		PhoneNumber: "5534988283531",
		Text:        "5 dias",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if len(businessEvents.events) != 1 {
		t.Fatalf("expected one health business event, got %d", len(businessEvents.events))
	}
	event := businessEvents.events[0]
	if event.Category != "health" || event.Subcategory != "mastitis_treatment" {
		t.Fatalf("unexpected event category/subcategory: %s/%s", event.Category, event.Subcategory)
	}
	if conversations.conversation.PendingConfirmationEventID != event.ID {
		t.Fatalf("expected pending confirmation event %q, got %q", event.ID, conversations.conversation.PendingConfirmationEventID)
	}
	if attrs := businessEvents.attributes[event.ID]; attrs["medicine"] != "Mastclin" || attrs["treatment_days"] != "5" || attrs["diagnosis_date"] != "hoje" {
		t.Fatalf("unexpected health attributes: %#v", attrs)
	}
	if _, found, _ := healthStates.GetByPhoneNumber(context.Background(), "5534988283531"); found {
		t.Fatalf("expected health treatment state to be deleted after completion")
	}
	if sender.sendCount != 1 {
		t.Fatalf("expected one confirmation prompt, got %d", sender.sendCount)
	}
	if !strings.Contains(sender.lastBody, "Medicamento: Mastclin") || !strings.Contains(sender.lastBody, "Dias de tratamento: 5") {
		t.Fatalf("unexpected confirmation prompt:\n%s", sender.lastBody)
	}
	if result.AssistantReplyKind != chatbot.ReplyKindConfirmation {
		t.Fatalf("expected confirmation reply kind, got %q", result.AssistantReplyKind)
	}
}

func TestCaptureServiceAsksForCorrelatedExpensesAfterHealthConfirmation(t *testing.T) {
	t.Parallel()

	conversations := &stubConversationRepository{
		conversation: domain.Conversation{
			ID:                         "conv-1",
			FarmID:                     "farm-1",
			PendingConfirmationEventID: "event-1",
		},
	}
	sourceMessages := &stubSourceMessageRepository{}
	businessEvents := &stubBusinessEventRepository{
		events: []domain.BusinessEvent{
			{
				ID:          "event-1",
				FarmID:      "farm-1",
				Category:    "health",
				Subcategory: "mastitis_treatment",
				AnimalCode:  "32",
				Description: "A vaca 32 esta com problema na teta T3 e nao pode tirar leite",
				Status:      domain.EventStatusDraft,
			},
		},
	}
	assistantMessages := &stubAssistantMessageRepository{}
	correlatedStates := &stubCorrelatedExpenseStateRepository{}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}

	service := NewCaptureService(
		nil,
		stubMessageProcessor{},
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5534988283531", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		conversations,
		sourceMessages,
		&stubTranscriptionRepository{},
		&stubInterpretationRunRepository{},
		businessEvents,
		assistantMessages,
	)
	service.SetCorrelatedExpenseStateRepository(correlatedStates)

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "confirm-health-1",
		PhoneNumber: "5534988283531",
		Text:        "sim",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if businessEvents.events[0].Status != domain.EventStatusConfirmed {
		t.Fatalf("expected health event to be confirmed, got %q", businessEvents.events[0].Status)
	}
	state, found, _ := correlatedStates.GetByPhoneNumber(context.Background(), "5534988283531")
	if !found {
		t.Fatalf("expected correlated expense state to be created")
	}
	if state.Step != domain.CorrelatedExpenseStepAwaitingDecision {
		t.Fatalf("expected awaiting decision step, got %q", state.Step)
	}
	if !strings.Contains(sender.lastBody, "Voce deseja lancar tambem os gastos") {
		t.Fatalf("unexpected follow-up prompt:\n%s", sender.lastBody)
	}
	if result.AssistantMessage.Text != sender.lastBody {
		t.Fatalf("expected assistant message to match outbound follow-up")
	}
}

func TestCaptureServiceCompletesCorrelatedExpenseFlow(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	conversations := &stubConversationRepository{
		conversation: domain.Conversation{
			ID:     "conv-1",
			FarmID: "farm-1",
		},
	}
	sourceMessages := &stubSourceMessageRepository{}
	interpretationRuns := &stubInterpretationRunRepository{}
	businessEvents := &stubBusinessEventRepository{}
	assistantMessages := &stubAssistantMessageRepository{}
	correlatedStates := &stubCorrelatedExpenseStateRepository{
		states: map[string]domain.CorrelatedExpenseState{
			"5534988283531": {
				PhoneNumber:     "5534988283531",
				FarmID:          "farm-1",
				RootEventID:     "event-health-1",
				RootCategory:    "health",
				RootSubcategory: "mastitis_treatment",
				AnimalCode:      "32",
				Description:     "Tratamento de mastite",
				OccurredAt:      &now,
				MedicineAmount:  float64Ptr(120),
				VetAmount:       float64Ptr(80),
				Step:            domain.CorrelatedExpenseStepAwaitingExamAmount,
				UpdatedAt:       now,
			},
		},
	}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}

	service := NewCaptureService(
		nil,
		&countingMessageProcessor{},
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5534988283531", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		conversations,
		sourceMessages,
		&stubTranscriptionRepository{},
		interpretationRuns,
		businessEvents,
		assistantMessages,
	)
	service.SetCorrelatedExpenseStateRepository(correlatedStates)

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "expense-flow-1",
		PhoneNumber: "5534988283531",
		Text:        "75",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if len(businessEvents.events) != 3 {
		t.Fatalf("expected three correlated expense events, got %d", len(businessEvents.events))
	}
	for _, event := range businessEvents.events {
		if event.Category != "finance" || event.Subcategory != "expense" {
			t.Fatalf("unexpected correlated expense category/subcategory: %s/%s", event.Category, event.Subcategory)
		}
		if event.Status != domain.EventStatusConfirmed || !event.ConfirmedByUser {
			t.Fatalf("expected correlated expense to be confirmed immediately")
		}
		if attrs := businessEvents.attributes[event.ID]; attrs["related_event_id"] != "event-health-1" {
			t.Fatalf("expected related_event_id on attributes, got %#v", attrs)
		}
	}
	if _, found, _ := correlatedStates.GetByPhoneNumber(context.Background(), "5534988283531"); found {
		t.Fatalf("expected correlated expense state to be deleted after completion")
	}
	if !strings.Contains(sender.lastBody, "Medicamento: R$ 120.00") || !strings.Contains(sender.lastBody, "Consulta veterinaria: R$ 80.00") || !strings.Contains(sender.lastBody, "Exames: R$ 75.00") {
		t.Fatalf("unexpected correlated expense summary:\n%s", sender.lastBody)
	}
	if result.AssistantMessage.Text != sender.lastBody {
		t.Fatalf("expected assistant message to match outbound summary")
	}
	if len(interpretationRuns.runs) != 3 {
		t.Fatalf("expected one interpretation run per correlated expense, got %d", len(interpretationRuns.runs))
	}
	if len(sourceMessages.messages) != 1 {
		t.Fatalf("expected one synthetic source message for correlated expenses, got %d", len(sourceMessages.messages))
	}
}

func TestCaptureServiceAnswersMilkWithdrawalQuery(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	conversations := &stubConversationRepository{
		conversation: domain.Conversation{
			ID:     "conv-1",
			FarmID: "farm-1",
		},
	}
	sourceMessages := &stubSourceMessageRepository{}
	assistantMessages := &stubAssistantMessageRepository{}
	businessEvents := &stubBusinessEventRepository{
		milkWithdrawal: []domain.MilkWithdrawalAnimal{
			{
				AnimalCode:    "32",
				Subcategory:   "mastitis_treatment",
				Description:   "Tratamento de mastite",
				AffectedTeats: []string{"T1", "T3"},
				OccurredAt:    &now,
				ActiveUntil:   timePtr(now.AddDate(0, 0, 5)),
			},
		},
	}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}

	service := NewCaptureService(
		nil,
		&countingMessageProcessor{},
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5534988283531", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		conversations,
		sourceMessages,
		&stubTranscriptionRepository{},
		&stubInterpretationRunRepository{},
		businessEvents,
		assistantMessages,
	)
	service.EnableBusinessQueryFlow()

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "query-1",
		PhoneNumber: "5534988283531",
		Text:        "Quais vacas nao podem tirar leite?",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if sender.sendCount != 1 {
		t.Fatalf("expected one outbound query reply, got %d", sender.sendCount)
	}
	if !strings.Contains(sender.lastBody, "Vacas com restricao de leite ativa") || !strings.Contains(sender.lastBody, "Animal: 32") {
		t.Fatalf("unexpected milk withdrawal reply:\n%s", sender.lastBody)
	}
	if result.AssistantMessage.Text != sender.lastBody {
		t.Fatalf("expected assistant message to match outbound query reply")
	}
	if len(sourceMessages.messages) != 1 {
		t.Fatalf("expected query source message to be persisted, got %d", len(sourceMessages.messages))
	}
	if len(assistantMessages.messages) != 1 {
		t.Fatalf("expected query assistant message to be persisted, got %d", len(assistantMessages.messages))
	}
}

func TestCaptureServiceAnswersRecentHealthTreatmentsQuery(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	conversations := &stubConversationRepository{
		conversation: domain.Conversation{
			ID:     "conv-1",
			FarmID: "farm-1",
		},
	}
	sourceMessages := &stubSourceMessageRepository{}
	assistantMessages := &stubAssistantMessageRepository{}
	businessEvents := &stubBusinessEventRepository{
		recentHealth: []domain.HealthTreatmentSummary{
			{
				AnimalCode:    "32",
				Subcategory:   "mastitis_treatment",
				Description:   "Tratamento de mastite",
				AffectedTeats: []string{"T1", "T3"},
				OccurredAt:    &now,
			},
			{
				AnimalCode:  "18",
				Subcategory: "hoof_treatment",
				Description: "Tratamento de casco",
				OccurredAt:  timePtr(now.AddDate(0, 0, -1)),
			},
		},
	}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}

	service := NewCaptureService(
		nil,
		&countingMessageProcessor{},
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5534988283531", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		conversations,
		sourceMessages,
		&stubTranscriptionRepository{},
		&stubInterpretationRunRepository{},
		businessEvents,
		assistantMessages,
	)
	service.EnableBusinessQueryFlow()

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "query-2",
		PhoneNumber: "5534988283531",
		Text:        "Quais foram os ultimos tratamentos?",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if sender.sendCount != 1 {
		t.Fatalf("expected one outbound treatments reply, got %d", sender.sendCount)
	}
	if !strings.Contains(sender.lastBody, "Ultimos tratamentos de saude") || !strings.Contains(sender.lastBody, "Animal: 32") || !strings.Contains(sender.lastBody, "Animal: 18") {
		t.Fatalf("unexpected recent treatments reply:\n%s", sender.lastBody)
	}
	if result.AssistantMessage.Text != sender.lastBody {
		t.Fatalf("expected assistant message to match outbound treatments reply")
	}
}

func TestCaptureServiceAnswersMedicineExpenseMonthQuery(t *testing.T) {
	t.Parallel()

	conversations := &stubConversationRepository{
		conversation: domain.Conversation{
			ID:     "conv-1",
			FarmID: "farm-1",
		},
	}
	sourceMessages := &stubSourceMessageRepository{}
	assistantMessages := &stubAssistantMessageRepository{}
	businessEvents := &stubBusinessEventRepository{
		medicineMonth: 275.50,
	}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}

	service := NewCaptureService(
		nil,
		&countingMessageProcessor{},
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5534988283531", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		conversations,
		sourceMessages,
		&stubTranscriptionRepository{},
		&stubInterpretationRunRepository{},
		businessEvents,
		assistantMessages,
	)
	service.EnableBusinessQueryFlow()

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "query-3",
		PhoneNumber: "5534988283531",
		Text:        "Quanto gastei com medicamento esse mes?",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if sender.sendCount != 1 {
		t.Fatalf("expected one outbound medicine expense reply, got %d", sender.sendCount)
	}
	if !strings.Contains(sender.lastBody, "Gasto com medicamento no mes") || !strings.Contains(sender.lastBody, "R$ 275.50") {
		t.Fatalf("unexpected medicine expense reply:\n%s", sender.lastBody)
	}
	if result.AssistantMessage.Text != sender.lastBody {
		t.Fatalf("expected assistant message to match outbound medicine expense reply")
	}
}

func TestCaptureServiceAnswersVetExpenseMonthQuery(t *testing.T) {
	t.Parallel()

	conversations := &stubConversationRepository{
		conversation: domain.Conversation{
			ID:     "conv-1",
			FarmID: "farm-1",
		},
	}
	sourceMessages := &stubSourceMessageRepository{}
	assistantMessages := &stubAssistantMessageRepository{}
	businessEvents := &stubBusinessEventRepository{
		vetMonth: 430.25,
	}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}

	service := NewCaptureService(
		nil,
		&countingMessageProcessor{},
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5534988283531", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		conversations,
		sourceMessages,
		&stubTranscriptionRepository{},
		&stubInterpretationRunRepository{},
		businessEvents,
		assistantMessages,
	)
	service.EnableBusinessQueryFlow()

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "query-4",
		PhoneNumber: "5534988283531",
		Text:        "Quanto gastei com veterinario esse mes?",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if sender.sendCount != 1 {
		t.Fatalf("expected one outbound vet expense reply, got %d", sender.sendCount)
	}
	if !strings.Contains(sender.lastBody, "Gasto com veterinario no mes") || !strings.Contains(sender.lastBody, "R$ 430.25") {
		t.Fatalf("unexpected vet expense reply:\n%s", sender.lastBody)
	}
	if result.AssistantMessage.Text != sender.lastBody {
		t.Fatalf("expected assistant message to match outbound vet expense reply")
	}
}

func TestCaptureServiceAnswersRecentPurchasesQuery(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	conversations := &stubConversationRepository{
		conversation: domain.Conversation{
			ID:     "conv-1",
			FarmID: "farm-1",
		},
	}
	sourceMessages := &stubSourceMessageRepository{}
	assistantMessages := &stubAssistantMessageRepository{}
	businessEvents := &stubBusinessEventRepository{
		recentPurchases: []domain.InputPurchaseSummary{
			{
				Description: "Comprei 10 sacos de racao por 850 reais",
				Amount:      float64Ptr(850),
				Quantity:    float64Ptr(10),
				Unit:        "saco",
				OccurredAt:  &now,
			},
			{
				Description: "Compramos adubo por 1200 reais",
				Amount:      float64Ptr(1200),
				OccurredAt:  timePtr(now.AddDate(0, 0, -2)),
			},
		},
	}
	sender := &stubChatMessageSender{}
	chatHistory := newStubChatConversationRepository()
	archive := &stubChatMessageArchive{}

	service := NewCaptureService(
		nil,
		&countingMessageProcessor{},
		sender,
		chatHistory,
		archive,
		NewRuleBasedInterpreter(),
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5534988283531", Status: "active"},
			},
		},
		nil,
		nil,
		nil,
		conversations,
		sourceMessages,
		&stubTranscriptionRepository{},
		&stubInterpretationRunRepository{},
		businessEvents,
		assistantMessages,
	)
	service.EnableBusinessQueryFlow()

	result, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "query-5",
		PhoneNumber: "5534988283531",
		Text:        "Quais foram as ultimas compras?",
		Type:        chat.MessageTypeText,
		Provider:    "whatsmeow",
	})
	if err != nil {
		t.Fatalf("ProcessIncomingMessage() error = %v", err)
	}

	if sender.sendCount != 1 {
		t.Fatalf("expected one outbound purchases reply, got %d", sender.sendCount)
	}
	if !strings.Contains(sender.lastBody, "Ultimas compras de insumos") || !strings.Contains(sender.lastBody, "Comprei 10 sacos de racao") || !strings.Contains(sender.lastBody, "Compramos adubo") {
		t.Fatalf("unexpected recent purchases reply:\n%s", sender.lastBody)
	}
	if result.AssistantMessage.Text != sender.lastBody {
		t.Fatalf("expected assistant message to match outbound purchases reply")
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
