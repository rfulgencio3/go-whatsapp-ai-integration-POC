package chat

import (
	"strings"
	"time"
	"unicode"
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
}

type MediaAttachment struct {
	URL         string
	ContentType string
	Filename    string
}

func NormalizePhoneNumber(value string) string {
	digits := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, strings.TrimSpace(value))

	digits = strings.TrimPrefix(digits, "00")
	switch len(digits) {
	case 10, 11:
		return "55" + digits
	default:
		return digits
	}
}
