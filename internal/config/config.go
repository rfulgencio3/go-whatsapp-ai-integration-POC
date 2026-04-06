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
	DefaultWebhookQueueWorkers      = 4
	DefaultWebhookQueueBufferSize   = 128
	DefaultWebhookQueueMaxRetries   = 3
	DefaultWebhookQueueRetryDelay   = 2 * time.Second
	DefaultTranscriptionMaxBytes    = 25 << 20
	DefaultChannelProvider          = "auto"
	DefaultWhatsmeowClientName      = "Chrome (Linux)"
	DefaultWhatsmeowPairMode        = "qr"
)

type Config struct {
	HTTPAddress              string
	RequestTimeout           time.Duration
	ConversationHistoryLimit int
	WhatsAppVerifyToken      string
	WhatsAppAppSecret        string
	WhatsAppAccessToken      string
	WhatsAppPhoneNumberID    string
	TwilioAccountSID         string
	TwilioAuthToken          string
	TwilioWhatsAppNumber     string
	TwilioWebhookBaseURL     string
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
	WebhookQueueWorkers      int
	WebhookQueueBufferSize   int
	WebhookQueueMaxRetries   int
	WebhookQueueRetryDelay   time.Duration
	DatabaseURL              string
	TranscriptionAPIBaseURL  string
	TranscriptionMaxBytes    int64
	ChannelProvider          string
	WhatsmeowStoreDSN        string
	WhatsmeowPairMode        string
	WhatsmeowPairPhone       string
	WhatsmeowClientName      string
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
		TwilioAccountSID:         strings.TrimSpace(os.Getenv("TWILIO_ACCOUNT_SID")),
		TwilioAuthToken:          strings.TrimSpace(os.Getenv("TWILIO_AUTH_TOKEN")),
		TwilioWhatsAppNumber:     strings.TrimSpace(os.Getenv("TWILIO_WHATSAPP_NUMBER")),
		TwilioWebhookBaseURL:     strings.TrimRight(strings.TrimSpace(os.Getenv("TWILIO_WEBHOOK_BASE_URL")), "/"),
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
		WebhookQueueWorkers:      getIntEnv("WEBHOOK_QUEUE_WORKERS", DefaultWebhookQueueWorkers),
		WebhookQueueBufferSize:   getIntEnv("WEBHOOK_QUEUE_BUFFER_SIZE", DefaultWebhookQueueBufferSize),
		WebhookQueueMaxRetries:   getIntEnvAllowZero("WEBHOOK_QUEUE_MAX_RETRIES", DefaultWebhookQueueMaxRetries),
		WebhookQueueRetryDelay:   getDurationEnv("WEBHOOK_QUEUE_RETRY_DELAY", DefaultWebhookQueueRetryDelay),
		DatabaseURL:              strings.TrimSpace(os.Getenv("DATABASE_URL")),
		TranscriptionAPIBaseURL:  strings.TrimRight(strings.TrimSpace(os.Getenv("TRANSCRIPTION_API_BASE_URL")), "/"),
		TranscriptionMaxBytes:    getInt64Env("TRANSCRIPTION_MAX_BYTES", DefaultTranscriptionMaxBytes),
		ChannelProvider:          strings.ToLower(getEnv("WHATSAPP_CHANNEL_PROVIDER", DefaultChannelProvider)),
		WhatsmeowStoreDSN:        strings.TrimSpace(getEnv("WHATSAPPMEOW_STORE_DSN", strings.TrimSpace(os.Getenv("DATABASE_URL")))),
		WhatsmeowPairMode:        strings.ToLower(getEnv("WHATSAPPMEOW_PAIR_MODE", DefaultWhatsmeowPairMode)),
		WhatsmeowPairPhone:       normalizePhoneNumber(strings.TrimSpace(os.Getenv("WHATSAPPMEOW_PAIR_PHONE"))),
		WhatsmeowClientName:      getEnv("WHATSAPPMEOW_CLIENT_NAME", DefaultWhatsmeowClientName),
	}
}

func (c Config) HasGeminiConfig() bool {
	return c.GeminiAPIKey != ""
}

func (c Config) HasWhatsAppSenderConfig() bool {
	return c.HasWhatsmeowConfig() || c.HasTwilioSenderConfig() || (c.WhatsAppAccessToken != "" && c.WhatsAppPhoneNumberID != "")
}

func (c Config) HasWhatsAppWebhookConfig() bool {
	return c.WhatsAppVerifyToken != "" || c.HasTwilioWebhookConfig()
}

func (c Config) HasTwilioSenderConfig() bool {
	return c.TwilioAccountSID != "" && c.TwilioAuthToken != "" && c.TwilioWhatsAppNumber != ""
}

func (c Config) HasTwilioWebhookConfig() bool {
	return c.TwilioAuthToken != ""
}

func (c Config) HasTranscriptionConfig() bool {
	return c.TranscriptionAPIBaseURL != ""
}

func (c Config) HasWhatsmeowConfig() bool {
	if c.ChannelProvider == "whatsmeow" {
		return c.WhatsmeowStoreDSN != ""
	}

	return false
}

func (c Config) MessagingProvider() string {
	switch {
	case c.HasWhatsmeowConfig():
		return "whatsmeow"
	case c.HasTwilioSenderConfig():
		return "twilio"
	case c.WhatsAppAccessToken != "" && c.WhatsAppPhoneNumberID != "":
		return "meta"
	default:
		return "noop"
	}
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

func getIntEnvAllowZero(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}

	return parsed
}

func getInt64Env(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
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
