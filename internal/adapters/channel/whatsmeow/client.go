package whatsmeow

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	wm "go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type Client struct {
	config         config.Config
	logger         *observability.Logger
	processor      chatbot.MessageProcessor
	mutex          sync.RWMutex
	client         *wm.Client
	started        bool
	pairCodeSent   bool
	eventHandlerID uint32
}

func New(cfg config.Config, logger *observability.Logger, processor chatbot.MessageProcessor) (*Client, error) {
	if logger == nil {
		logger = observability.NewLogger()
	}
	if !cfg.HasWhatsmeowConfig() {
		return nil, fmt.Errorf("whatsmeow is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	dbLog := waLog.Stdout("WhatsmeowDB", "INFO", false)
	container, err := sqlstore.New(ctx, "postgres", cfg.WhatsmeowStoreDSN, dbLog)
	if err != nil {
		return nil, fmt.Errorf("initialize whatsmeow sql store: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("load whatsmeow device store: %w", err)
	}

	clientLog := waLog.Stdout("Whatsmeow", "INFO", false)
	waClient := wm.NewClient(deviceStore, clientLog)

	instance := &Client{
		config:    cfg,
		logger:    logger,
		processor: processor,
		client:    waClient,
	}
	instance.eventHandlerID = waClient.AddEventHandler(instance.handleEvent)

	return instance, nil
}

func (c *Client) Start(ctx context.Context) error {
	c.mutex.Lock()
	if c.started {
		c.mutex.Unlock()
		return nil
	}
	c.started = true
	c.mutex.Unlock()

	if c.client.Store.ID == nil {
		qrChan, err := c.client.GetQRChannel(ctx)
		if err != nil {
			return fmt.Errorf("create whatsmeow qr channel: %w", err)
		}
		if err := c.client.Connect(); err != nil {
			return fmt.Errorf("connect whatsmeow client: %w", err)
		}
		go c.consumeQRChannel(ctx, qrChan)
		c.logger.Info("whatsmeow waiting for device pairing", map[string]any{
			"pair_mode": c.config.WhatsmeowPairMode,
		})
		return nil
	}

	if err := c.client.Connect(); err != nil {
		return fmt.Errorf("connect whatsmeow client: %w", err)
	}

	c.logger.Info("whatsmeow session connected", map[string]any{
		"jid": c.client.Store.ID.String(),
	})
	return nil
}

func (c *Client) SetProcessor(processor chatbot.MessageProcessor) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.processor = processor
}

func (c *Client) Stop() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.client == nil {
		return
	}
	if c.eventHandlerID != 0 {
		c.client.RemoveEventHandler(c.eventHandlerID)
	}
	c.client.Disconnect()
	c.started = false
}

func (c *Client) SendTextMessage(ctx context.Context, phoneNumber, body string) error {
	c.mutex.RLock()
	client := c.client
	c.mutex.RUnlock()

	if client == nil {
		return fmt.Errorf("whatsmeow client is not initialized")
	}

	jid := types.NewJID(chat.NormalizePhoneNumber(phoneNumber), types.DefaultUserServer)
	_, err := client.SendMessage(ctx, jid, &waProto.Message{
		Conversation: proto.String(body),
	})
	if err != nil {
		return fmt.Errorf("send whatsmeow text message: %w", err)
	}

	return nil
}

func (c *Client) consumeQRChannel(ctx context.Context, qrChan <-chan wm.QRChannelItem) {
	for item := range qrChan {
		switch item.Event {
		case wm.QRChannelEventCode:
			if c.shouldUsePhonePairing() {
				code, err := c.client.PairPhone(ctx, c.config.WhatsmeowPairPhone, true, wm.PairClientChrome, c.config.WhatsmeowClientName)
				if err != nil {
					c.logger.Error("whatsmeow phone pairing failed", map[string]any{"error": err.Error()})
				} else {
					c.pairCodeSent = true
					c.logger.Info("whatsmeow pairing code generated", map[string]any{
						"pair_phone": c.config.WhatsmeowPairPhone,
						"pair_code":  code,
					})
				}
				continue
			}

			c.logger.Info("whatsmeow qr code generated", map[string]any{
				"qr_code": item.Code,
			})
		default:
			c.logger.Info("whatsmeow pairing event", map[string]any{
				"event": item.Event,
				"code":  item.Code,
			})
		}
	}
}

func (c *Client) shouldUsePhonePairing() bool {
	return !c.pairCodeSent &&
		c.config.WhatsmeowPairMode == "code" &&
		strings.TrimSpace(c.config.WhatsmeowPairPhone) != ""
}

func (c *Client) handleEvent(evt any) {
	switch event := evt.(type) {
	case *events.Message:
		c.handleIncomingMessage(event)
	case *events.Connected:
		c.logger.Info("whatsmeow connected", nil)
	case *events.Disconnected:
		c.logger.Info("whatsmeow disconnected", nil)
	case *events.PairSuccess:
		c.logger.Info("whatsmeow pairing success", map[string]any{
			"jid":           event.ID.String(),
			"business_name": event.BusinessName,
			"platform":      event.Platform,
		})
	case *events.PairError:
		c.logger.Error("whatsmeow pairing failed", map[string]any{
			"jid":           event.ID.String(),
			"business_name": event.BusinessName,
			"platform":      event.Platform,
			"error":         event.Error.Error(),
		})
	}
}

func (c *Client) handleIncomingMessage(event *events.Message) {
	if event == nil || event.Info.IsFromMe || event.Message == nil {
		return
	}

	text, messageType := extractInboundContent(event.Message)
	incoming := chat.IncomingMessage{
		MessageID:   event.Info.ID,
		PhoneNumber: event.Info.Sender.User,
		Text:        text,
		Type:        messageType,
		Provider:    "whatsmeow",
	}

	c.mutex.RLock()
	processor := c.processor
	c.mutex.RUnlock()

	if processor == nil {
		c.logger.Info("whatsmeow inbound message received without processor", map[string]any{
			"phone_number": incoming.PhoneNumber,
			"message_id":   incoming.MessageID,
			"type":         incoming.Type,
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err := processor.ProcessIncomingMessage(ctx, incoming); err != nil {
		c.logger.Error("whatsmeow inbound processing failed", map[string]any{
			"phone_number": incoming.PhoneNumber,
			"message_id":   incoming.MessageID,
			"type":         incoming.Type,
			"error":        err.Error(),
		})
	}
}

func extractInboundContent(message *waProto.Message) (string, chat.MessageType) {
	switch {
	case strings.TrimSpace(message.GetConversation()) != "":
		return strings.TrimSpace(message.GetConversation()), chat.MessageTypeText
	case message.GetExtendedTextMessage() != nil && strings.TrimSpace(message.GetExtendedTextMessage().GetText()) != "":
		return strings.TrimSpace(message.GetExtendedTextMessage().GetText()), chat.MessageTypeText
	case message.GetImageMessage() != nil:
		return strings.TrimSpace(message.GetImageMessage().GetCaption()), chat.MessageTypeImage
	case message.GetDocumentMessage() != nil:
		return strings.TrimSpace(message.GetDocumentMessage().GetCaption()), chat.MessageTypeDocument
	case message.GetAudioMessage() != nil:
		return "", chat.MessageTypeAudio
	default:
		return "", chat.MessageTypeUnsupported
	}
}
