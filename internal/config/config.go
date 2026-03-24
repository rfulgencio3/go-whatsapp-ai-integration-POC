package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultHTTPAddress              = ":8081"
	DefaultGeminiModel              = "gemini-2.0-flash"
	DefaultSystemPrompt             = "You are a WhatsApp support assistant. Reply in Brazilian Portuguese with concise, useful, and context-aware answers."
	DefaultRequestTimeout           = 20 * time.Second
	DefaultConversationHistoryLimit = 12
)

type Config struct {
	HTTPAddress              string
	RequestTimeout           time.Duration
	ConversationHistoryLimit int
	WhatsAppVerifyToken      string
	WhatsAppAppSecret        string
	WhatsAppAccessToken      string
	WhatsAppPhoneNumberID    string
	GeminiAPIKey             string
	GeminiModel              string
	SystemPrompt             string
	AllowedPhoneNumber       string
}

func Load() Config {
	return Config{
		HTTPAddress:              getEnv("HTTP_ADDRESS", DefaultHTTPAddress),
		RequestTimeout:           getDurationEnv("REQUEST_TIMEOUT", DefaultRequestTimeout),
		ConversationHistoryLimit: getIntEnv("CONVERSATION_HISTORY_LIMIT", DefaultConversationHistoryLimit),
		WhatsAppVerifyToken:      strings.TrimSpace(os.Getenv("WHATSAPP_VERIFY_TOKEN")),
		WhatsAppAppSecret:        strings.TrimSpace(os.Getenv("WHATSAPP_APP_SECRET")),
		WhatsAppAccessToken:      strings.TrimSpace(os.Getenv("WHATSAPP_ACCESS_TOKEN")),
		WhatsAppPhoneNumberID:    strings.TrimSpace(os.Getenv("WHATSAPP_PHONE_NUMBER_ID")),
		GeminiAPIKey:             strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		GeminiModel:              getEnv("GEMINI_MODEL", DefaultGeminiModel),
		SystemPrompt:             getEnv("SYSTEM_PROMPT", DefaultSystemPrompt),
		AllowedPhoneNumber:       normalizePhoneNumber(strings.TrimSpace(os.Getenv("ALLOWED_PHONE_NUMBER"))),
	}
}

func (c Config) HasGeminiConfig() bool {
	return c.GeminiAPIKey != ""
}

func (c Config) HasWhatsAppSenderConfig() bool {
	return c.WhatsAppAccessToken != "" && c.WhatsAppPhoneNumberID != ""
}

func (c Config) HasWhatsAppWebhookConfig() bool {
	return c.WhatsAppVerifyToken != ""
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getIntEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}

func normalizePhoneNumber(value string) string {
	replacer := strings.NewReplacer(" ", "", "+", "", "-", "", "(", "", ")", "")
	return replacer.Replace(value)
}
