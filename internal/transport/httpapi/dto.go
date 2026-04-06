package httpapi

import (
	"strings"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

type HealthResponse struct {
	Status                    string `json:"status"`
	WhatsAppSenderConfigured  bool   `json:"whatsapp_sender_configured"`
	WhatsAppWebhookConfigured bool   `json:"whatsapp_webhook_configured"`
	TwilioConfigured          bool   `json:"twilio_configured"`
	TranscriptionConfigured   bool   `json:"transcription_configured"`
	MessagingProvider         string `json:"messaging_provider"`
	GeminiConfigured          bool   `json:"gemini_configured"`
	GeminiModel               string `json:"gemini_model"`
}

type WebhookResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type SimulationRequest struct {
	PhoneNumber string `json:"phone_number"`
	Message     string `json:"message"`
}

type SimulationResponse struct {
	PhoneNumber  string `json:"phone_number"`
	InputMessage string `json:"input_message"`
	ReplyMessage string `json:"reply_message"`
}

type WhatsAppWebhookNotification struct {
	Entries []WhatsAppWebhookEntry `json:"entry"`
}

type WhatsAppWebhookEntry struct {
	Changes []WhatsAppWebhookChange `json:"changes"`
}

type WhatsAppWebhookChange struct {
	Value WhatsAppWebhookValue `json:"value"`
}

type WhatsAppWebhookValue struct {
	Contacts []WhatsAppContact `json:"contacts"`
	Messages []WhatsAppMessage `json:"messages"`
}

type WhatsAppContact struct {
	WhatsAppID string `json:"wa_id"`
}

type WhatsAppMessage struct {
	ID   string               `json:"id"`
	From string               `json:"from"`
	Type string               `json:"type"`
	Text WhatsAppTextEnvelope `json:"text"`
}

type WhatsAppTextEnvelope struct {
	Body string `json:"body"`
}

func (n WhatsAppWebhookNotification) ExtractIncomingMessages() []chat.IncomingMessage {
	var messages []chat.IncomingMessage

	for _, entry := range n.Entries {
		for _, change := range entry.Changes {
			for _, incomingMessage := range change.Value.Messages {
				if incomingMessage.Type != "text" {
					continue
				}

				messageID := strings.TrimSpace(incomingMessage.ID)
				if messageID == "" {
					continue
				}

				body := strings.TrimSpace(incomingMessage.Text.Body)
				if body == "" {
					continue
				}

				phoneNumber := chat.NormalizePhoneNumber(incomingMessage.From)
				if phoneNumber == "" && len(change.Value.Contacts) > 0 {
					phoneNumber = chat.NormalizePhoneNumber(change.Value.Contacts[0].WhatsAppID)
				}
				if phoneNumber == "" {
					continue
				}

				messages = append(messages, chat.IncomingMessage{
					MessageID:   messageID,
					PhoneNumber: phoneNumber,
					Text:        body,
					Type:        chat.MessageTypeText,
					Provider:    "meta",
				})
			}
		}
	}

	return messages
}
