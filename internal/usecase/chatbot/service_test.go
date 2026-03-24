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
	return &stubConversationRepository{store: make(map[string][]chat.Message)}
}

func (s *stubConversationRepository) GetMessages(_ context.Context, phoneNumber string) ([]chat.Message, error) {
	return append([]chat.Message(nil), s.store[phoneNumber]...), nil
}

func (s *stubConversationRepository) AppendMessage(_ context.Context, phoneNumber string, message chat.Message) error {
	s.store[phoneNumber] = append(s.store[phoneNumber], message)
	return nil
}

type stubMessageArchive struct {
	recorded []chat.Message
	err      error
}

func (s *stubMessageArchive) RecordMessage(_ context.Context, _ string, message chat.Message) error {
	if s.err != nil {
		return s.err
	}

	s.recorded = append(s.recorded, message)
	return nil
}

func TestBuildReply(t *testing.T) {
	repository := newStubConversationRepository()
	archive := &stubMessageArchive{}
	sender := &stubMessageSender{}
	service := NewService("", stubReplyGenerator{reply: "assistant reply"}, sender, repository, archive)

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

	if len(archive.recorded) != 2 {
		t.Fatalf("expected 2 archived messages, got %d", len(archive.recorded))
	}
}

func TestProcessIncomingMessageRejectsPhoneNumberOutsideAllowList(t *testing.T) {
	repository := newStubConversationRepository()
	archive := &stubMessageArchive{}
	sender := &stubMessageSender{}
	service := NewService("5511888888888", stubReplyGenerator{reply: "assistant reply"}, sender, repository, archive)

	err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		PhoneNumber: "5511999999999",
		Text:        "hello",
	})

	if !errors.Is(err, chat.ErrPhoneNumberNotAllowed) {
		t.Fatalf("expected ErrPhoneNumberNotAllowed, got %v", err)
	}
}

func TestBuildReplyReturnsArchiveError(t *testing.T) {
	repository := newStubConversationRepository()
	archive := &stubMessageArchive{err: errors.New("archive failure")}
	sender := &stubMessageSender{}
	service := NewService("", stubReplyGenerator{reply: "assistant reply"}, sender, repository, archive)

	_, err := service.BuildReply(context.Background(), "5511999999999", "hello")
	if err == nil || err.Error() != "archive failure" {
		t.Fatalf("expected archive failure, got %v", err)
	}
}
