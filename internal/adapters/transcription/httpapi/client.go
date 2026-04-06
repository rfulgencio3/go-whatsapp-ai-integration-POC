package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
)

type Result struct {
	ID            string  `json:"Id"`
	Transcript    string  `json:"transcript"`
	Language      string  `json:"language"`
	AudioDuration float64 `json:"audioDuration"`
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(httpClient *http.Client, baseURL string) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
	}
}

func (c *Client) Transcribe(ctx context.Context, fileName, contentType string, data []byte) (Result, error) {
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	part, err := writer.CreateFormFile("audio", fallbackFileName(fileName, contentType))
	if err != nil {
		return Result{}, fmt.Errorf("create transcription form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return Result{}, fmt.Errorf("write transcription form file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return Result{}, fmt.Errorf("close transcription multipart writer: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/transcribe", &requestBody)
	if err != nil {
		return Result{}, fmt.Errorf("create transcription request: %w", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := c.httpClient.Do(request)
	if err != nil {
		return Result{}, fmt.Errorf("transcription request failed: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return Result{}, fmt.Errorf("read transcription response: %w", err)
	}
	if response.StatusCode >= http.StatusMultipleChoices {
		return Result{}, fmt.Errorf("transcription api returned status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var result Result
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return Result{}, fmt.Errorf("decode transcription response: %w", err)
	}
	return result, nil
}

func fallbackFileName(fileName, contentType string) string {
	trimmed := strings.TrimSpace(fileName)
	if trimmed != "" {
		return path.Base(trimmed)
	}

	switch {
	case strings.Contains(contentType, "ogg"):
		return "audio.ogg"
	case strings.Contains(contentType, "mpeg"):
		return "audio.mp3"
	case strings.Contains(contentType, "wav"):
		return "audio.wav"
	default:
		return "audio.bin"
	}
}
