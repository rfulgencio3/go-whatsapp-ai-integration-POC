package whatsmeow

import (
	"context"
	"testing"

	transcriptionhttpapi "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/transcription/httpapi"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	wm "go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type stubMediaDownloader struct {
	payload []byte
	err     error
	calls   int
}

func (s *stubMediaDownloader) Download(_ context.Context, _ wm.DownloadableMessage) ([]byte, error) {
	s.calls++
	return s.payload, s.err
}

type stubAudioTranscriber struct {
	result      transcriptionhttpapi.Result
	err         error
	calls       int
	lastName    string
	lastType    string
	lastPayload []byte
}

func (s *stubAudioTranscriber) Transcribe(_ context.Context, fileName, contentType string, data []byte) (transcriptionhttpapi.Result, error) {
	s.calls++
	s.lastName = fileName
	s.lastType = contentType
	s.lastPayload = append([]byte(nil), data...)
	return s.result, s.err
}

func TestBuildIncomingMessageExtractsAudioMetadata(t *testing.T) {
	t.Parallel()

	event := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.NewJID("5511999999999", types.DefaultUserServer),
			},
			ID: types.MessageID("wamid-audio-1"),
		},
		Message: &waProto.Message{
			AudioMessage: &waProto.AudioMessage{
				Mimetype: proto.String("audio/ogg; codecs=opus"),
				Seconds:  proto.Uint32(12),
			},
		},
	}

	incoming := buildIncomingMessage(event)

	if incoming.Type != chat.MessageTypeAudio {
		t.Fatalf("expected audio message type, got %q", incoming.Type)
	}
	if incoming.AudioDurationSeconds != 12 {
		t.Fatalf("expected audio duration 12, got %v", incoming.AudioDurationSeconds)
	}
	if len(incoming.MediaAttachments) != 1 {
		t.Fatalf("expected one media attachment, got %d", len(incoming.MediaAttachments))
	}
	if incoming.MediaAttachments[0].Filename != "audio.ogg" {
		t.Fatalf("expected audio.ogg filename, got %q", incoming.MediaAttachments[0].Filename)
	}
}

func TestEnrichIncomingMessageTranscribesAudio(t *testing.T) {
	t.Parallel()

	downloader := &stubMediaDownloader{payload: []byte("audio-bytes")}
	transcriber := &stubAudioTranscriber{result: transcriptionhttpapi.Result{
		ID:            "tr-123",
		Transcript:    "comprei 10 sacos de racao por 850 reais",
		Language:      "pt-BR",
		AudioDuration: 8.5,
	}}
	client := &Client{
		config: config.Config{
			TranscriptionMaxBytes: 1024,
		},
		logger:      observability.NewLogger(),
		downloader:  downloader,
		transcriber: transcriber,
	}
	message := &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			Mimetype:   proto.String("audio/ogg"),
			FileLength: proto.Uint64(20),
			Seconds:    proto.Uint32(7),
		},
	}
	incoming := chat.IncomingMessage{
		MessageID:   "wamid-audio-2",
		PhoneNumber: "5511999999999",
		Type:        chat.MessageTypeAudio,
		Provider:    "whatsmeow",
		MediaAttachments: []chat.MediaAttachment{{
			ContentType: "audio/ogg",
			Filename:    "voice.ogg",
		}},
		AudioDurationSeconds: 7,
	}

	enriched := client.enrichIncomingMessage(context.Background(), incoming, message)

	if downloader.calls != 1 {
		t.Fatalf("expected one download call, got %d", downloader.calls)
	}
	if transcriber.calls != 1 {
		t.Fatalf("expected one transcribe call, got %d", transcriber.calls)
	}
	if transcriber.lastName != "voice.ogg" {
		t.Fatalf("expected voice.ogg filename, got %q", transcriber.lastName)
	}
	if enriched.Text != "comprei 10 sacos de racao por 850 reais" {
		t.Fatalf("unexpected transcript: %q", enriched.Text)
	}
	if enriched.TranscriptionID != "tr-123" {
		t.Fatalf("expected transcription id tr-123, got %q", enriched.TranscriptionID)
	}
	if enriched.TranscriptionLanguage != "pt-BR" {
		t.Fatalf("expected pt-BR language, got %q", enriched.TranscriptionLanguage)
	}
	if enriched.AudioDurationSeconds != 8.5 {
		t.Fatalf("expected overwritten duration 8.5, got %v", enriched.AudioDurationSeconds)
	}
}

func TestEnrichIncomingMessageSkipsOversizedAudio(t *testing.T) {
	t.Parallel()

	downloader := &stubMediaDownloader{payload: []byte("audio-bytes")}
	transcriber := &stubAudioTranscriber{}
	client := &Client{
		config: config.Config{
			TranscriptionMaxBytes: 4,
		},
		logger:      observability.NewLogger(),
		downloader:  downloader,
		transcriber: transcriber,
	}
	message := &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			Mimetype:   proto.String("audio/ogg"),
			FileLength: proto.Uint64(10),
		},
	}
	incoming := chat.IncomingMessage{
		MessageID:   "wamid-audio-3",
		PhoneNumber: "5511999999999",
		Type:        chat.MessageTypeAudio,
		Provider:    "whatsmeow",
	}

	enriched := client.enrichIncomingMessage(context.Background(), incoming, message)

	if downloader.calls != 0 {
		t.Fatalf("expected no download for oversized audio, got %d", downloader.calls)
	}
	if transcriber.calls != 0 {
		t.Fatalf("expected no transcription for oversized audio, got %d", transcriber.calls)
	}
	if enriched.Text != "" {
		t.Fatalf("expected empty text when transcription is skipped, got %q", enriched.Text)
	}
}

func TestEnrichIncomingMessageSkipsAudioLongerThanSupportedDuration(t *testing.T) {
	t.Parallel()

	downloader := &stubMediaDownloader{payload: []byte("audio-bytes")}
	transcriber := &stubAudioTranscriber{}
	client := &Client{
		config: config.Config{
			TranscriptionMaxBytes:    1024,
			TranscriptionMaxAudioSec: 30,
		},
		logger:      observability.NewLogger(),
		downloader:  downloader,
		transcriber: transcriber,
	}
	message := &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			Mimetype:   proto.String("audio/ogg"),
			FileLength: proto.Uint64(10),
			Seconds:    proto.Uint32(31),
		},
	}
	incoming := chat.IncomingMessage{
		MessageID:            "wamid-audio-4",
		PhoneNumber:          "5511999999999",
		Type:                 chat.MessageTypeAudio,
		Provider:             "whatsmeow",
		AudioDurationSeconds: 31,
	}

	enriched := client.enrichIncomingMessage(context.Background(), incoming, message)

	if downloader.calls != 0 {
		t.Fatalf("expected no download for oversized audio duration, got %d", downloader.calls)
	}
	if transcriber.calls != 0 {
		t.Fatalf("expected no transcription for oversized audio duration, got %d", transcriber.calls)
	}
	if !enriched.AudioTooLong {
		t.Fatalf("expected audio to be flagged as too long")
	}
}
