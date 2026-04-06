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
}

func (s stubMessageProcessor) ProcessIncomingMessage(_ context.Context, _ chat.IncomingMessage) (chatbot.ProcessResult, error) {
	return s.result, s.err
}

type stubFarmMembershipRepository struct {
	memberships []domain.FarmMembership
	err         error
}

func (s stubFarmMembershipRepository) FindActiveByPhoneNumber(_ context.Context, _ string) ([]domain.FarmMembership, error) {
	return s.memberships, s.err
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
	assistantMessages := &stubAssistantMessageRepository{}
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
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5511999999999", Status: "active"},
			},
		},
		conversations,
		sourceMessages,
		transcriptions,
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
	if len(assistantMessages.messages) != 1 {
		t.Fatalf("expected one assistant message, got %d", len(assistantMessages.messages))
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
	assistantMessages := &stubAssistantMessageRepository{}
	service := NewCaptureService(
		nil,
		stubMessageProcessor{result: chatbot.ProcessResult{Duplicate: true}},
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5511999999999", Status: "active"},
			},
		},
		conversations,
		sourceMessages,
		transcriptions,
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
	assistantMessages := &stubAssistantMessageRepository{}
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
		stubFarmMembershipRepository{
			memberships: []domain.FarmMembership{
				{ID: "membership-1", FarmID: "farm-1", PhoneNumber: "5511999999999", Status: "active"},
			},
		},
		conversations,
		sourceMessages,
		transcriptions,
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
	if got := transcriptions.transcriptions[0].ProviderRef; got != "external-transcription-id" {
		t.Fatalf("expected transcription provider ref to be preserved, got %q", got)
	}
	if got := transcriptions.transcriptions[0].SourceMessageID; got != sourceMessages.messages[0].ID {
		t.Fatalf("expected transcription to link source message, got %q", got)
	}
}
