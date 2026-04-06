package noop

import (
	"context"
	"strings"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

type Preprocessor struct{}

func NewPreprocessor() *Preprocessor {
	return &Preprocessor{}
}

func (p *Preprocessor) Prepare(_ context.Context, message chat.IncomingMessage) (chat.IncomingMessage, error) {
	message.Text = strings.TrimSpace(message.Text)
	if message.Type == "" {
		message.Type = chat.MessageTypeText
	}
	return message, nil
}
