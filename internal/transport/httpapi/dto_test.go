package httpapi

import "testing"

func TestExtractIncomingMessagesFiltersUnsupportedPayloads(t *testing.T) {
	notification := WhatsAppWebhookNotification{
		Entries: []WhatsAppWebhookEntry{{
			Changes: []WhatsAppWebhookChange{{
				Value: WhatsAppWebhookValue{
					Contacts: []WhatsAppContact{{WhatsAppID: "5511999999999"}},
					Messages: []WhatsAppMessage{
						{ID: "", From: "5511999999999", Type: "text", Text: WhatsAppTextEnvelope{Body: "ignored because id is missing"}},
						{ID: "wamid.audio", From: "5511999999999", Type: "audio", Text: WhatsAppTextEnvelope{Body: ""}},
						{ID: "wamid.valid", From: "5511999999999", Type: "text", Text: WhatsAppTextEnvelope{Body: "hello"}},
					},
				},
			}},
		}},
	}

	messages := notification.ExtractIncomingMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 extracted message, got %d", len(messages))
	}

	if messages[0].MessageID != "wamid.valid" {
		t.Fatalf("expected message id to be preserved, got %q", messages[0].MessageID)
	}
}
