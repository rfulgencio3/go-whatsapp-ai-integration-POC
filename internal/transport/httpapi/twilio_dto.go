package httpapi

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

func extractIncomingTwilioMessage(values url.Values) chat.IncomingMessage {
	messageID := strings.TrimSpace(values.Get("MessageSid"))
	phoneNumber := chat.NormalizePhoneNumber(values.Get("WaId"))
	if phoneNumber == "" {
		phoneNumber = chat.NormalizePhoneNumber(stripWhatsAppPrefix(values.Get("From")))
	}

	body := strings.TrimSpace(values.Get("Body"))
	attachments := extractTwilioMediaAttachments(values)
	messageType := inferTwilioMessageType(body, attachments)

	return chat.IncomingMessage{
		MessageID:        messageID,
		PhoneNumber:      phoneNumber,
		Text:             body,
		Type:             messageType,
		Provider:         "twilio",
		MediaAttachments: attachments,
	}
}

func extractTwilioMediaAttachments(values url.Values) []chat.MediaAttachment {
	numMedia, err := strconv.Atoi(strings.TrimSpace(values.Get("NumMedia")))
	if err != nil || numMedia <= 0 {
		return nil
	}

	attachments := make([]chat.MediaAttachment, 0, numMedia)
	for index := 0; index < numMedia; index++ {
		mediaURL := strings.TrimSpace(values.Get("MediaUrl" + strconv.Itoa(index)))
		if mediaURL == "" {
			continue
		}
		attachments = append(attachments, chat.MediaAttachment{
			URL:         mediaURL,
			ContentType: strings.TrimSpace(values.Get("MediaContentType" + strconv.Itoa(index))),
		})
	}
	return attachments
}

func inferTwilioMessageType(body string, attachments []chat.MediaAttachment) chat.MessageType {
	if strings.TrimSpace(body) != "" {
		return chat.MessageTypeText
	}
	if len(attachments) == 0 {
		return chat.MessageTypeUnsupported
	}

	contentType := strings.ToLower(strings.TrimSpace(attachments[0].ContentType))
	switch {
	case strings.HasPrefix(contentType, "audio/"):
		return chat.MessageTypeAudio
	case strings.HasPrefix(contentType, "image/"):
		return chat.MessageTypeImage
	case strings.HasPrefix(contentType, "application/"):
		return chat.MessageTypeDocument
	default:
		return chat.MessageTypeUnsupported
	}
}

func stripWhatsAppPrefix(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(trimmed), "whatsapp:") {
		return strings.TrimSpace(trimmed[len("whatsapp:"):])
	}
	return trimmed
}
