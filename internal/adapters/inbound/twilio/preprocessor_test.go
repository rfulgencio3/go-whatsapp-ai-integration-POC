package twilio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	transcriptionhttpapi "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/transcription/httpapi"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

type stubAudioTranscriber struct {
	result transcriptionhttpapi.Result
}

func (s stubAudioTranscriber) Transcribe(_ context.Context, _ string, _ string, data []byte) (transcriptionhttpapi.Result, error) {
	if len(data) == 0 {
		return transcriptionhttpapi.Result{}, nil
	}
	return s.result, nil
}

func TestPreprocessorPrepareTranscribesAudioAttachment(t *testing.T) {
	mediaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "AC123" || password != "secret" {
			t.Fatalf("expected basic auth credentials to be forwarded")
		}
		_, _ = w.Write([]byte("audio-bytes"))
	}))
	defer mediaServer.Close()

	preprocessor := NewPreprocessor(
		mediaServer.Client(),
		"AC123",
		"secret",
		1024,
		stubAudioTranscriber{result: transcriptionhttpapi.Result{
			ID:            "tr-1",
			Transcript:    "audio transcribed",
			Language:      "pt-BR",
			AudioDuration: 5.2,
		}},
	)

	message, err := preprocessor.Prepare(context.Background(), chat.IncomingMessage{
		MessageID:   "SM123",
		PhoneNumber: "5511999999999",
		Type:        chat.MessageTypeAudio,
		MediaAttachments: []chat.MediaAttachment{{
			URL:         mediaServer.URL + "/media/1",
			ContentType: "audio/ogg",
		}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if message.Text != "audio transcribed" {
		t.Fatalf("expected transcript to populate message text, got %q", message.Text)
	}
	if message.TranscriptionID != "tr-1" {
		t.Fatalf("expected transcription id to be propagated, got %q", message.TranscriptionID)
	}
}

func TestPreprocessorPrepareRejectsUnsupportedMedia(t *testing.T) {
	preprocessor := NewPreprocessor(&http.Client{}, "AC123", "secret", 1024, nil)

	_, err := preprocessor.Prepare(context.Background(), chat.IncomingMessage{
		MessageID:   "SMimage",
		PhoneNumber: "5511999999999",
		Type:        chat.MessageTypeImage,
		MediaAttachments: []chat.MediaAttachment{{
			URL:         "https://example.com/media/1",
			ContentType: "image/jpeg",
		}},
	})
	if err != chat.ErrUnsupportedMessageType {
		t.Fatalf("expected ErrUnsupportedMessageType, got %v", err)
	}
}
