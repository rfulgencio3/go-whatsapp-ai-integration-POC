package twilio

import (
	"context"
	"fmt"
	"strings"

	twilio "github.com/twilio/twilio-go"
	twilioapi "github.com/twilio/twilio-go/rest/api/v2010"
)

type Client struct {
	client             *twilio.RestClient
	whatsAppSenderAddr string
}

func NewClient(accountSID, authToken, whatsAppNumber string) *Client {
	return &Client{
		client: twilio.NewRestClientWithParams(twilio.ClientParams{
			Username: strings.TrimSpace(accountSID),
			Password: strings.TrimSpace(authToken),
		}),
		whatsAppSenderAddr: normalizeWhatsAppAddress(whatsAppNumber),
	}
}

func (c *Client) SendTextMessage(_ context.Context, phoneNumber, body string) error {
	params := &twilioapi.CreateMessageParams{}
	params.SetTo(normalizeWhatsAppAddress(phoneNumber))
	params.SetFrom(c.whatsAppSenderAddr)
	params.SetBody(body)

	if _, err := c.client.Api.CreateMessage(params); err != nil {
		return fmt.Errorf("send twilio whatsapp message: %w", err)
	}

	return nil
}

func normalizeWhatsAppAddress(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "whatsapp:") {
		return "whatsapp:" + strings.TrimSpace(trimmed[len("whatsapp:"):])
	}
	if strings.HasPrefix(trimmed, "+") {
		return "whatsapp:" + trimmed
	}
	return "whatsapp:+" + trimmed
}
