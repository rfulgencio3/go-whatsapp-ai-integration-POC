package whatsmeow

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	transcriptionhttpapi "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/transcription/httpapi"
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
	downloader     mediaDownloader
	transcriber    AudioTranscriber
	started        bool
	pairCodeSent   bool
	eventHandlerID uint32
}

type AudioTranscriber interface {
	Transcribe(ctx context.Context, fileName, contentType string, data []byte) (transcriptionhttpapi.Result, error)
}

type mediaDownloader interface {
	Download(ctx context.Context, msg wm.DownloadableMessage) ([]byte, error)
}

func New(cfg config.Config, logger *observability.Logger, processor chatbot.MessageProcessor, transcriber AudioTranscriber) (*Client, error) {
	if logger == nil {
		logger = observability.NewLogger()
	}
	if !cfg.HasWhatsmeowConfig() {
		return nil, fmt.Errorf("whatsmeow is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	dbLog := waLog.Stdout("WhatsmeowDB", "INFO", false)
	database, err := sql.Open("pgx", cfg.WhatsmeowStoreDSN)
	if err != nil {
		return nil, fmt.Errorf("open whatsmeow sql database: %w", err)
	}
	defer func() {
		if err != nil {
			_ = database.Close()
		}
	}()

	container := sqlstore.NewWithDB(database, "postgres", dbLog)
	if err = container.Upgrade(ctx); err != nil {
		return nil, fmt.Errorf("initialize whatsmeow sql store: failed to upgrade database: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("load whatsmeow device store: %w", err)
	}

	clientLog := waLog.Stdout("Whatsmeow", "INFO", false)
	waClient := wm.NewClient(deviceStore, clientLog)

	instance := &Client{
		config:      cfg,
		logger:      logger,
		processor:   processor,
		client:      waClient,
		downloader:  waClient,
		transcriber: transcriber,
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

	incoming := buildIncomingMessage(event)

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

	incoming = c.enrichIncomingMessage(ctx, incoming, event.Message)

	if _, err := processor.ProcessIncomingMessage(ctx, incoming); err != nil {
		c.logger.Error("whatsmeow inbound processing failed", map[string]any{
			"phone_number": incoming.PhoneNumber,
			"message_id":   incoming.MessageID,
			"type":         incoming.Type,
			"error":        err.Error(),
		})
	}
}

func buildIncomingMessage(event *events.Message) chat.IncomingMessage {
	if event == nil || event.Message == nil {
		return chat.IncomingMessage{Provider: "whatsmeow"}
	}

	text, messageType, attachments, duration := extractInboundContent(event.Message)
	return chat.IncomingMessage{
		MessageID:            event.Info.ID,
		PhoneNumber:          event.Info.Sender.User,
		Text:                 text,
		Type:                 messageType,
		Provider:             "whatsmeow",
		MediaAttachments:     attachments,
		AudioDurationSeconds: duration,
	}
}

func (c *Client) enrichIncomingMessage(ctx context.Context, incoming chat.IncomingMessage, message *waProto.Message) chat.IncomingMessage {
	if message == nil || incoming.Type != chat.MessageTypeAudio || c.transcriber == nil || c.downloader == nil {
		return incoming
	}

	audio := message.GetAudioMessage()
	if audio == nil {
		return incoming
	}
	if c.config.TranscriptionMaxBytes > 0 && int64(audio.GetFileLength()) > c.config.TranscriptionMaxBytes {
		c.logger.Error("whatsmeow inbound audio exceeds transcription max bytes", map[string]any{
			"message_id": eventMessageID(incoming),
			"size_bytes": audio.GetFileLength(),
		})
		return incoming
	}

	payload, err := c.downloader.Download(ctx, audio)
	if err != nil {
		c.logger.Error("whatsmeow audio download failed", map[string]any{
			"message_id": eventMessageID(incoming),
			"error":      err.Error(),
		})
		return incoming
	}
	if c.config.TranscriptionMaxBytes > 0 && int64(len(payload)) > c.config.TranscriptionMaxBytes {
		c.logger.Error("whatsmeow inbound audio payload exceeds transcription max bytes", map[string]any{
			"message_id": eventMessageID(incoming),
			"size_bytes": len(payload),
		})
		return incoming
	}

	attachment := firstAttachment(incoming.MediaAttachments)
	result, err := c.transcriber.Transcribe(ctx, fallbackAudioFileName(attachment), attachment.ContentType, payload)
	if err != nil {
		c.logger.Error("whatsmeow audio transcription failed", map[string]any{
			"message_id": eventMessageID(incoming),
			"error":      err.Error(),
		})
		return incoming
	}

	incoming.Text = strings.TrimSpace(result.Transcript)
	incoming.TranscriptionID = strings.TrimSpace(result.ID)
	incoming.TranscriptionLanguage = strings.TrimSpace(result.Language)
	if result.AudioDuration > 0 {
		incoming.AudioDurationSeconds = result.AudioDuration
	}

	return incoming
}

func extractInboundContent(message *waProto.Message) (string, chat.MessageType, []chat.MediaAttachment, float64) {
	switch {
	case strings.TrimSpace(message.GetConversation()) != "":
		return strings.TrimSpace(message.GetConversation()), chat.MessageTypeText, nil, 0
	case message.GetExtendedTextMessage() != nil && strings.TrimSpace(message.GetExtendedTextMessage().GetText()) != "":
		return strings.TrimSpace(message.GetExtendedTextMessage().GetText()), chat.MessageTypeText, nil, 0
	case message.GetImageMessage() != nil:
		image := message.GetImageMessage()
		return strings.TrimSpace(image.GetCaption()), chat.MessageTypeImage, []chat.MediaAttachment{{
			ContentType: strings.TrimSpace(image.GetMimetype()),
		}}, 0
	case message.GetDocumentMessage() != nil:
		document := message.GetDocumentMessage()
		return strings.TrimSpace(document.GetCaption()), chat.MessageTypeDocument, []chat.MediaAttachment{{
			ContentType: strings.TrimSpace(document.GetMimetype()),
			Filename:    strings.TrimSpace(document.GetFileName()),
		}}, 0
	case message.GetAudioMessage() != nil:
		audio := message.GetAudioMessage()
		return "", chat.MessageTypeAudio, []chat.MediaAttachment{{
			ContentType: strings.TrimSpace(audio.GetMimetype()),
			Filename:    fallbackAudioFileName(chat.MediaAttachment{ContentType: strings.TrimSpace(audio.GetMimetype())}),
		}}, float64(audio.GetSeconds())
	default:
		return "", chat.MessageTypeUnsupported, nil, 0
	}
}

func firstAttachment(attachments []chat.MediaAttachment) chat.MediaAttachment {
	if len(attachments) == 0 {
		return chat.MediaAttachment{}
	}

	return attachments[0]
}

func fallbackAudioFileName(attachment chat.MediaAttachment) string {
	if strings.TrimSpace(attachment.Filename) != "" {
		return strings.TrimSpace(attachment.Filename)
	}

	contentType := strings.ToLower(strings.TrimSpace(attachment.ContentType))
	switch {
	case strings.Contains(contentType, "ogg"):
		return "audio.ogg"
	case strings.Contains(contentType, "mpeg"), strings.Contains(contentType, "mp3"):
		return "audio.mp3"
	case strings.Contains(contentType, "wav"):
		return "audio.wav"
	default:
		return "audio.bin"
	}
}

func eventMessageID(message chat.IncomingMessage) string {
	return strings.TrimSpace(message.MessageID)
}
