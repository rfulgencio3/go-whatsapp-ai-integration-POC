package channel

import (
	"context"
	"time"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
)

type InboundMessage struct {
	Provider          string
	ProviderMessageID string
	SenderPhoneNumber string
	MessageType       agro.MessageType
	Text              string
	MediaURL          string
	MediaContentType  string
	MediaFilename     string
	ReceivedAt        time.Time
}

type OutboundMessage struct {
	RecipientPhoneNumber string
	ReplyType            agro.ReplyType
	Body                 string
}

type InboundHandler interface {
	HandleInbound(ctx context.Context, message InboundMessage) error
}

type Sender interface {
	SendText(ctx context.Context, message OutboundMessage) (providerMessageID string, err error)
}
