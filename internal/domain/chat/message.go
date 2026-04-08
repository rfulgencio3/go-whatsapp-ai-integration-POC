package chat

import (
	"time"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/common"
)

type MessageRole string
type MessageType string

const (
	UserRole      MessageRole = "user"
	AssistantRole MessageRole = "assistant"

	MessageTypeText        MessageType = "text"
	MessageTypeAudio       MessageType = "audio"
	MessageTypeImage       MessageType = "image"
	MessageTypeDocument    MessageType = "document"
	MessageTypeUnsupported MessageType = "unsupported"
)

type Message struct {
	Role                  MessageRole
	Text                  string
	CreatedAt             time.Time
	Type                  MessageType
	Provider              string
	ProviderMessageID     string
	MediaURL              string
	MediaContentType      string
	MediaFilename         string
	TranscriptionID       string
	TranscriptionLanguage string
	AudioDurationSeconds  float64
}

type IncomingMessage struct {
	MessageID             string
	PhoneNumber           string
	Text                  string
	Type                  MessageType
	Provider              string
	MediaAttachments      []MediaAttachment
	TranscriptionID       string
	TranscriptionLanguage string
	AudioDurationSeconds  float64
	AudioTooLong          bool
}

type MediaAttachment struct {
	URL         string
	ContentType string
	Filename    string
}

func NormalizePhoneNumber(value string) string {
	return common.NormalizePhoneNumber(value)
}
