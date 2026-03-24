package chat

import (
	"strings"
	"time"
)

type MessageRole string

const (
	UserRole      MessageRole = "user"
	AssistantRole MessageRole = "assistant"
)

type Message struct {
	Role      MessageRole
	Text      string
	CreatedAt time.Time
}

type IncomingMessage struct {
	MessageID   string
	PhoneNumber string
	Text        string
}

func NormalizePhoneNumber(value string) string {
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer(" ", "", "+", "", "-", "", "(", "", ")", "")
	return replacer.Replace(value)
}
