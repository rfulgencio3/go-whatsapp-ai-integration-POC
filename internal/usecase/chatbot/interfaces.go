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
	AppendMessage(phoneNumber string, message chat.Message) []chat.Message
}
