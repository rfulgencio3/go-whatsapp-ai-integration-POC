package twilio

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	transcriptionhttpapi "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/transcription/httpapi"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

type AudioTranscriber interface {
	Transcribe(ctx context.Context, fileName, contentType string, data []byte) (transcriptionhttpapi.Result, error)
}

type Preprocessor struct {
	httpClient  *http.Client
	accountSID  string
	authToken   string
	maxBytes    int64
	transcriber AudioTranscriber
}

func NewPreprocessor(httpClient *http.Client, accountSID, authToken string, maxBytes int64, transcriber AudioTranscriber) *Preprocessor {
	return &Preprocessor{
		httpClient:  httpClient,
		accountSID:  strings.TrimSpace(accountSID),
		authToken:   strings.TrimSpace(authToken),
		maxBytes:    maxBytes,
		transcriber: transcriber,
	}
}

func (p *Preprocessor) Prepare(ctx context.Context, message chat.IncomingMessage) (chat.IncomingMessage, error) {
	message.Text = strings.TrimSpace(message.Text)
	if message.Provider == "" {
		message.Provider = "twilio"
	}
	if message.Text != "" {
		if message.Type == "" {
			message.Type = chat.MessageTypeText
		}
		return message, nil
	}

	audio, ok := firstAudioAttachment(message.MediaAttachments)
	if !ok || p.transcriber == nil {
		return message, chat.ErrUnsupportedMessageType
	}

	payload, err := p.downloadMedia(ctx, audio.URL)
	if err != nil {
		return message, err
	}

	result, err := p.transcriber.Transcribe(ctx, fileNameForAttachment(audio), audio.ContentType, payload)
	if err != nil {
		return message, fmt.Errorf("transcribe inbound audio: %w", err)
	}

	message.Type = chat.MessageTypeAudio
	message.Text = strings.TrimSpace(result.Transcript)
	message.TranscriptionID = strings.TrimSpace(result.ID)
	message.TranscriptionLanguage = strings.TrimSpace(result.Language)
	message.AudioDurationSeconds = result.AudioDuration
	if message.Text == "" {
		return message, fmt.Errorf("transcribe inbound audio: empty transcript")
	}
	return message, nil
}

func (p *Preprocessor) downloadMedia(ctx context.Context, mediaURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create media download request: %w", err)
	}
	request.SetBasicAuth(p.accountSID, p.authToken)

	response, err := p.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download media: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return nil, fmt.Errorf("download media returned status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	reader := io.LimitReader(response.Body, p.maxBytes+1)
	payload, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read media payload: %w", err)
	}
	if int64(len(payload)) > p.maxBytes {
		return nil, fmt.Errorf("media payload exceeds %d bytes", p.maxBytes)
	}
	return payload, nil
}

func firstAudioAttachment(attachments []chat.MediaAttachment) (chat.MediaAttachment, bool) {
	for _, attachment := range attachments {
		contentType := strings.ToLower(strings.TrimSpace(attachment.ContentType))
		if strings.HasPrefix(contentType, "audio/") {
			return attachment, true
		}
	}
	return chat.MediaAttachment{}, false
}

func fileNameForAttachment(attachment chat.MediaAttachment) string {
	if trimmed := strings.TrimSpace(attachment.Filename); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(attachment.URL); trimmed != "" {
		if fileName := path.Base(trimmed); fileName != "." && fileName != "/" {
			return fileName
		}
	}
	return "audio.bin"
}
