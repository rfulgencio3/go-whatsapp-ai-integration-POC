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

type sequenceReplyGenerator struct {
	replies []string
	errors  []error
	calls   int
}

func (s *sequenceReplyGenerator) GenerateReply(_ context.Context, _ []chat.Message) (string, error) {
	index := s.calls
	s.calls++
	var reply string
	if index < len(s.replies) {
		reply = s.replies[index]
	}
	var err error
	if index < len(s.errors) {
		err = s.errors[index]
	}
	return reply, err
}

type stubMessageSender struct {
	lastPhoneNumber string
	lastBody        string
	sendCount       int
	err             error
}

func (s *stubMessageSender) SendTextMessage(_ context.Context, phoneNumber, body string) error {
	s.lastPhoneNumber = phoneNumber
	s.lastBody = body
	s.sendCount++
	return s.err
}

type stubIncomingPreprocessor struct {
	prepared chat.IncomingMessage
	err      error
}

func (s *stubIncomingPreprocessor) Prepare(_ context.Context, message chat.IncomingMessage) (chat.IncomingMessage, error) {
	if s.err != nil {
		return chat.IncomingMessage{}, s.err
	}
	if s.prepared.PhoneNumber == "" {
		return message, nil
	}
	return s.prepared, nil
}

type stubConversationRepository struct{ store map[string][]chat.Message }

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

type stubMessageDeduplicator struct {
	processing map[string]struct{}
	processed  map[string]struct{}
	acquireErr error
	markErr    error
	releaseErr error
}

func newStubMessageDeduplicator() *stubMessageDeduplicator {
	return &stubMessageDeduplicator{processing: make(map[string]struct{}), processed: make(map[string]struct{})}
}
func (s *stubMessageDeduplicator) Acquire(_ context.Context, messageID string) (bool, error) {
	if s.acquireErr != nil {
		return false, s.acquireErr
	}
	if _, ok := s.processing[messageID]; ok {
		return false, nil
	}
	if _, ok := s.processed[messageID]; ok {
		return false, nil
	}
	s.processing[messageID] = struct{}{}
	return true, nil
}
func (s *stubMessageDeduplicator) MarkProcessed(_ context.Context, messageID string) error {
	if s.markErr != nil {
		return s.markErr
	}
	delete(s.processing, messageID)
	s.processed[messageID] = struct{}{}
	return nil
}
func (s *stubMessageDeduplicator) Release(_ context.Context, messageID string) error {
	if s.releaseErr != nil {
		return s.releaseErr
	}
	delete(s.processing, messageID)
	return nil
}

func TestBuildReply(t *testing.T) {
	repository := newStubConversationRepository()
	archive := &stubMessageArchive{}
	sender := &stubMessageSender{}
	deduplicator := newStubMessageDeduplicator()
	service := NewService("", stubReplyGenerator{reply: "assistant reply"}, sender, nil, repository, archive, deduplicator)

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
	deduplicator := newStubMessageDeduplicator()
	service := NewService("5511888888888", stubReplyGenerator{reply: "assistant reply"}, sender, nil, repository, archive, deduplicator)

	_, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{MessageID: "wamid.1", PhoneNumber: "5511999999999", Text: "hello"})
	if !errors.Is(err, chat.ErrPhoneNumberNotAllowed) {
		t.Fatalf("expected ErrPhoneNumberNotAllowed, got %v", err)
	}
}

func TestBuildReplyReturnsArchiveError(t *testing.T) {
	repository := newStubConversationRepository()
	archive := &stubMessageArchive{err: errors.New("archive failure")}
	sender := &stubMessageSender{}
	deduplicator := newStubMessageDeduplicator()
	service := NewService("", stubReplyGenerator{reply: "assistant reply"}, sender, nil, repository, archive, deduplicator)

	_, err := service.BuildReply(context.Background(), "5511999999999", "hello")
	if err == nil || err.Error() != "archive failure" {
		t.Fatalf("expected archive failure, got %v", err)
	}
}

func TestProcessIncomingMessageSkipsDuplicateMessageID(t *testing.T) {
	repository := newStubConversationRepository()
	archive := &stubMessageArchive{}
	sender := &stubMessageSender{}
	deduplicator := newStubMessageDeduplicator()
	service := NewService("", stubReplyGenerator{reply: "assistant reply"}, sender, nil, repository, archive, deduplicator)
	message := chat.IncomingMessage{MessageID: "wamid.123", PhoneNumber: "5511999999999", Text: "hello"}

	firstResult, err := service.ProcessIncomingMessage(context.Background(), message)
	if err != nil {
		t.Fatalf("expected nil error on first processing, got %v", err)
	}
	if firstResult.Duplicate {
		t.Fatalf("expected first result to not be duplicate")
	}

	secondResult, err := service.ProcessIncomingMessage(context.Background(), message)
	if err != nil {
		t.Fatalf("expected nil error on duplicate processing, got %v", err)
	}
	if !secondResult.Duplicate {
		t.Fatalf("expected duplicate result on second processing")
	}
	if sender.sendCount != 1 {
		t.Fatalf("expected 1 outbound message, got %d", sender.sendCount)
	}
	messages := repository.store["5511999999999"]
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages in history, got %d", len(messages))
	}
	if _, ok := deduplicator.processed[message.MessageID]; !ok {
		t.Fatalf("expected message id %q to be marked as processed", message.MessageID)
	}
}

func TestProcessIncomingMessageReleasesMessageIDOnFailure(t *testing.T) {
	repository := newStubConversationRepository()
	archive := &stubMessageArchive{}
	sender := &stubMessageSender{}
	deduplicator := newStubMessageDeduplicator()
	replyGenerator := &sequenceReplyGenerator{replies: []string{"", "assistant reply"}, errors: []error{errors.New("gemini failure"), nil}}
	service := NewService("", replyGenerator, sender, nil, repository, archive, deduplicator)
	message := chat.IncomingMessage{MessageID: "wamid.retry", PhoneNumber: "5511999999999", Text: "hello"}

	firstResult, firstErr := service.ProcessIncomingMessage(context.Background(), message)
	if firstErr == nil || firstErr.Error() != "gemini failure" {
		t.Fatalf("expected gemini failure on first processing, got %v", firstErr)
	}
	if firstResult.Duplicate {
		t.Fatalf("expected failed first result to not be duplicate")
	}
	if _, ok := deduplicator.processing[message.MessageID]; ok {
		t.Fatalf("expected message id %q to be released after failure", message.MessageID)
	}
	if len(repository.store["5511999999999"]) != 0 {
		t.Fatalf("expected no history to be persisted on failed processing")
	}

	secondResult, err := service.ProcessIncomingMessage(context.Background(), message)
	if err != nil {
		t.Fatalf("expected nil error on retry, got %v", err)
	}
	if secondResult.Duplicate {
		t.Fatalf("expected retry result to not be duplicate")
	}
	if sender.sendCount != 1 {
		t.Fatalf("expected 1 outbound message after retry, got %d", sender.sendCount)
	}
	if _, ok := deduplicator.processed[message.MessageID]; !ok {
		t.Fatalf("expected message id %q to be marked as processed after retry", message.MessageID)
	}
}

func TestProcessIncomingMessageDoesNotPersistOnSendFailure(t *testing.T) {
	repository := newStubConversationRepository()
	archive := &stubMessageArchive{}
	sender := &stubMessageSender{err: errors.New("send failure")}
	deduplicator := newStubMessageDeduplicator()
	service := NewService("", stubReplyGenerator{reply: "assistant reply"}, sender, nil, repository, archive, deduplicator)

	_, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{MessageID: "wamid.sendfail", PhoneNumber: "5511999999999", Text: "hello"})
	if err == nil || err.Error() != "send failure" {
		t.Fatalf("expected send failure, got %v", err)
	}
	if len(repository.store["5511999999999"]) != 0 {
		t.Fatalf("expected no persisted history after send failure")
	}
	if len(archive.recorded) != 0 {
		t.Fatalf("expected no archived messages after send failure")
	}
}

func TestProcessIncomingMessageUsesPreprocessedAudioTranscript(t *testing.T) {
	repository := newStubConversationRepository()
	archive := &stubMessageArchive{}
	sender := &stubMessageSender{}
	deduplicator := newStubMessageDeduplicator()
	preprocessor := &stubIncomingPreprocessor{prepared: chat.IncomingMessage{
		MessageID:             "SMaudio",
		PhoneNumber:           "5511999999999",
		Text:                  "transcribed audio",
		Type:                  chat.MessageTypeAudio,
		Provider:              "twilio",
		TranscriptionID:       "transcription-123",
		TranscriptionLanguage: "pt-BR",
		AudioDurationSeconds:  9.5,
		MediaAttachments: []chat.MediaAttachment{{
			URL:         "https://api.twilio.com/media/1",
			ContentType: "audio/ogg",
		}},
	}}
	service := NewService("", stubReplyGenerator{reply: "assistant reply"}, sender, preprocessor, repository, archive, deduplicator)

	_, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "SMaudio",
		PhoneNumber: "5511999999999",
		Type:        chat.MessageTypeAudio,
		Provider:    "twilio",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	messages := repository.store["5511999999999"]
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages in history, got %d", len(messages))
	}
	if messages[0].Type != chat.MessageTypeAudio {
		t.Fatalf("expected user message type audio, got %q", messages[0].Type)
	}
	if messages[0].TranscriptionID != "transcription-123" {
		t.Fatalf("expected transcription id to be preserved, got %q", messages[0].TranscriptionID)
	}
	if sender.lastBody != "assistant reply" {
		t.Fatalf("expected assistant reply to be sent, got %q", sender.lastBody)
	}
}

func TestProcessIncomingMessageRepliesToUnsupportedMedia(t *testing.T) {
	repository := newStubConversationRepository()
	archive := &stubMessageArchive{}
	sender := &stubMessageSender{}
	deduplicator := newStubMessageDeduplicator()
	preprocessor := &stubIncomingPreprocessor{err: chat.ErrUnsupportedMessageType}
	service := NewService("", stubReplyGenerator{reply: "assistant reply"}, sender, preprocessor, repository, archive, deduplicator)

	_, err := service.ProcessIncomingMessage(context.Background(), chat.IncomingMessage{
		MessageID:   "SMimage",
		PhoneNumber: "5511999999999",
		Type:        chat.MessageTypeImage,
		Provider:    "twilio",
		MediaAttachments: []chat.MediaAttachment{{
			URL:         "https://api.twilio.com/media/2",
			ContentType: "image/jpeg",
		}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if sender.lastBody != unsupportedInboundReply {
		t.Fatalf("expected unsupported media reply, got %q", sender.lastBody)
	}
	if len(archive.recorded) != 2 {
		t.Fatalf("expected unsupported flow to be archived, got %d messages", len(archive.recorded))
	}
}
