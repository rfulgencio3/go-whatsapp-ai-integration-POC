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
	DefaultRedisConversationTTL     = 24 * time.Hour
	DefaultRedisKeyPrefix           = "chat:history"
	DefaultWebhookIdempotencyTTL    = 72 * time.Hour
	DefaultWebhookProcessingTTL     = 2 * time.Minute
	DefaultRedisIdempotencyPrefix   = "webhook:idempotency"
	DefaultRedisProcessingPrefix    = "webhook:processing"
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
	RedisURL                 string
	RedisConversationTTL     time.Duration
	RedisKeyPrefix           string
	WebhookIdempotencyTTL    time.Duration
	WebhookProcessingTTL     time.Duration
	RedisIdempotencyPrefix   string
	RedisProcessingPrefix    string
	DatabaseURL              string
}

func Load() Config {
	return Config{
		HTTPAddress:              resolveHTTPAddress(),
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
		RedisURL:                 strings.TrimSpace(os.Getenv("REDIS_URL")),
		RedisConversationTTL:     getDurationEnv("REDIS_CONVERSATION_TTL", DefaultRedisConversationTTL),
		RedisKeyPrefix:           getEnv("REDIS_KEY_PREFIX", DefaultRedisKeyPrefix),
		WebhookIdempotencyTTL:    getDurationEnv("WEBHOOK_IDEMPOTENCY_TTL", DefaultWebhookIdempotencyTTL),
		WebhookProcessingTTL:     getDurationEnv("WEBHOOK_PROCESSING_TTL", DefaultWebhookProcessingTTL),
		RedisIdempotencyPrefix:   getEnv("REDIS_IDEMPOTENCY_PREFIX", DefaultRedisIdempotencyPrefix),
		RedisProcessingPrefix:    getEnv("REDIS_PROCESSING_PREFIX", DefaultRedisProcessingPrefix),
		DatabaseURL:              strings.TrimSpace(os.Getenv("DATABASE_URL")),
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

func (c Config) HasRedisConfig() bool {
	return c.RedisURL != ""
}

func (c Config) HasDatabaseConfig() bool {
	return c.DatabaseURL != ""
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

func resolveHTTPAddress() string {
	if address := strings.TrimSpace(os.Getenv("HTTP_ADDRESS")); address != "" {
		return address
	}

	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		if strings.HasPrefix(port, ":") {
			return port
		}

		return ":" + port
	}

	return DefaultHTTPAddress
}

func normalizePhoneNumber(value string) string {
	replacer := strings.NewReplacer(" ", "", "+", "", "-", "", "(", "", ")", "")
	return replacer.Replace(value)
}
