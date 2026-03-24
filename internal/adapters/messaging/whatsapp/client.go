package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	httpClient    *http.Client
	accessToken   string
	phoneNumberID string
}

func NewClient(httpClient *http.Client, accessToken, phoneNumberID string) *Client {
	return &Client{
		httpClient:    httpClient,
		accessToken:   strings.TrimSpace(accessToken),
		phoneNumberID: strings.TrimSpace(phoneNumberID),
	}
}

func (c *Client) SendTextMessage(ctx context.Context, phoneNumber, body string) error {
	requestBody := sendMessageRequest{
		MessagingProduct: "whatsapp",
		To:               phoneNumber,
		Type:             "text",
		Text: textPayload{
			Body: body,
		},
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://graph.facebook.com/v22.0/%s/messages", c.phoneNumberID)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.accessToken)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("whatsapp api returned status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return nil
}

type sendMessageRequest struct {
	MessagingProduct string      `json:"messaging_product"`
	To               string      `json:"to"`
	Type             string      `json:"type"`
	Text             textPayload `json:"text"`
}

type textPayload struct {
	Body string `json:"body"`
}
