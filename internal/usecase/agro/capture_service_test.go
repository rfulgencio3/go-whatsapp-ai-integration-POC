package agro

import (
	"context"
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
		phoneContexts,
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
		phoneContexts,
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
