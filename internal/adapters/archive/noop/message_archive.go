package noop

import (
	"context"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

type MessageArchive struct{}

func NewMessageArchive() *MessageArchive {
	return &MessageArchive{}
}

func (a *MessageArchive) RecordMessage(_ context.Context, _ string, _ chat.Message) error {
	return nil
}
