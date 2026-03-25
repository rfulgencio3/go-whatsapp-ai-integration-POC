package chatbot

import (
	"context"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

type ReplyGenerator interface {
	GenerateReply(ctx context.Context, history []chat.Message) (string, error)
}

type MessageSender interface {
	SendTextMessage(ctx context.Context, phoneNumber, body string) error
}

type ConversationRepository interface {
	GetMessages(ctx context.Context, phoneNumber string) ([]chat.Message, error)
	AppendMessage(ctx context.Context, phoneNumber string, message chat.Message) error
}

type MessageArchive interface {
	RecordMessage(ctx context.Context, phoneNumber string, message chat.Message) error
}

type MessageDeduplicator interface {
	Acquire(ctx context.Context, messageID string) (bool, error)
	MarkProcessed(ctx context.Context, messageID string) error
	Release(ctx context.Context, messageID string) error
}

type MessageProcessor interface {
	ProcessIncomingMessage(ctx context.Context, message chat.IncomingMessage) (ProcessResult, error)
}

type MessageQueue interface {
	Enqueue(ctx context.Context, message chat.IncomingMessage) error
}
