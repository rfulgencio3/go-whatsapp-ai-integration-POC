package chatbot

import (
	"context"
	"errors"
	"testing"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

type stubReplyGenerator struct {
	reply string
	err   error
}

func (s stubReplyGenerator) GenerateReply(_ context.Context, _ []chat.Message) (string, error) {
	return s.reply, s.err
}

type stubMessageSender struct {
	lastPhoneNumber string
	lastBody        string
	err             error
}

func (s *stubMessageSender) SendTextMessage(_ context.Context, phoneNumber, body string) error {
	s.lastPhoneNumber = phoneNumber
	s.lastBody = body
	return s.err
}

type stubConversationRepository struct {
	store map[string][]chat.Message
}

func newStubConversationRepository() *stubConversationRepository {
	return &stubConversationRepository{
		store: make(map[string][]chat.Message),
	}
}

func (s *stubConversationRepository) AppendMessage(phoneNumber string, message chat.Message) []chat.Message {
	s.store[phoneNumber] = append(s.store[phoneNumber], message)
	return append([]chat.Message(nil), s.store[phoneNumber]...)
}

func TestBuildReply(t *testing.T) {
	repository := newStubConversationRepository()
	sender := &stubMessageSender{}
	service := NewService("", stubReplyGenerator{reply: "assistant reply"}, sender, repository)

	reply, err := service.BuildReply(context.Background(), "+55 (11) 99999-9999", "hello")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if reply != "assistant reply" {
		t.Fatalf("expected reply to be recorded, got %q", reply)
	}

	messages := repository.store["5511999999999"]
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages in history, got %d", len(messages))
	}

	if messages[0].Role != chat.UserRole || messages[1].Role != chat.AssistantRole {
		t.Fatalf("expected user/assistant history, got %+v", messages)
	}
}

func TestProcessIncomingMessageRejectsPhoneNumberOutsideAllowList(t *testing.T) {
	repository := newStubConversationRepository()
	sender := &stubMessageSender{}
	service := NewService("5511888888888", stubReplyGenerator{reply: "assistant reply"}, sender, repository)

	err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		PhoneNumber: "5511999999999",
		Text:        "hello",
	})

	if !errors.Is(err, chat.ErrPhoneNumberNotAllowed) {
		t.Fatalf("expected ErrPhoneNumberNotAllowed, got %v", err)
	}
}
