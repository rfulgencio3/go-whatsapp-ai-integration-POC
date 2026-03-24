package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
)

type Client struct {
	httpClient        *http.Client
	apiKey            string
	model             string
	systemInstruction string
}

func NewClient(httpClient *http.Client, apiKey, model, systemInstruction string) *Client {
	return &Client{
		httpClient:        httpClient,
		apiKey:            strings.TrimSpace(apiKey),
		model:             strings.TrimSpace(model),
		systemInstruction: strings.TrimSpace(systemInstruction),
	}
}

func (c *Client) GenerateReply(ctx context.Context, history []chat.Message) (string, error) {
	if c.apiKey == "" {
		return "", errors.New("gemini api key is not configured")
	}

	requestBody := generateContentRequest{
		SystemInstruction: partContainer{
			Parts: []part{{Text: c.systemInstruction}},
		},
		Contents: make([]content, 0, len(history)),
	}

	for _, message := range history {
		role := "user"
		if message.Role == chat.AssistantRole {
			role = "model"
		}

		requestBody.Contents = append(requestBody.Contents, content{
			Role:  role,
			Parts: []part{{Text: message.Text}},
		})
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		c.model,
		c.apiKey,
	)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}

	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	if response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("gemini returned status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed generateContentResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}

	reply := strings.TrimSpace(parsed.FirstText())
	if reply == "" {
		return "", errors.New("gemini returned an empty response")
	}

	return reply, nil
}

type generateContentRequest struct {
	SystemInstruction partContainer `json:"system_instruction"`
	Contents          []content     `json:"contents"`
}

type partContainer struct {
	Parts []part `json:"parts"`
}

type content struct {
	Role  string `json:"role"`
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content content `json:"content"`
	} `json:"candidates"`
}

func (r generateContentResponse) FirstText() string {
	for _, candidate := range r.Candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				return part.Text
			}
		}
	}

	return ""
}
