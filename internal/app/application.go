package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	nooparchive "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/archive/noop"
	postgresarchive "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/archive/postgres"
	memoryidempotency "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/idempotency/memory"
	redisidempotency "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/idempotency/redis"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/messaging/noop"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/messaging/whatsapp"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/reply/fallback"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/reply/gemini"
	memoryrepo "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/repository/memory"
	redisrepo "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/repository/redis"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/transport/httpapi"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type Application struct {
	server *http.Server
	logger *observability.Logger
}

func New(cfg config.Config) (*Application, error) {
	logger := observability.NewLogger()
	metrics := observability.NewMetrics()
	httpClient := &http.Client{Timeout: cfg.RequestTimeout}
	startupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conversationRepository := chatbot.ConversationRepository(memoryrepo.NewConversationRepository(cfg.ConversationHistoryLimit))
	if cfg.HasRedisConfig() {
		redisConversationRepository, err := redisrepo.NewConversationRepository(startupContext, cfg.RedisURL, cfg.ConversationHistoryLimit, cfg.RedisConversationTTL, cfg.RedisKeyPrefix)
		if err != nil {
			return nil, fmt.Errorf("initialize redis conversation repository: %w", err)
		}
		conversationRepository = redisConversationRepository
	}

	messageArchive := chatbot.MessageArchive(nooparchive.NewMessageArchive())
	if cfg.HasDatabaseConfig() {
		postgresMessageArchive, err := postgresarchive.NewMessageArchive(startupContext, cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("initialize postgres message archive: %w", err)
		}
		messageArchive = postgresMessageArchive
	}

	messageDeduplicator := chatbot.MessageDeduplicator(memoryidempotency.NewStore(cfg.WebhookIdempotencyTTL, cfg.WebhookProcessingTTL))
	if cfg.HasRedisConfig() {
		redisMessageDeduplicator, err := redisidempotency.NewStore(startupContext, cfg.RedisURL, cfg.WebhookIdempotencyTTL, cfg.WebhookProcessingTTL, cfg.RedisIdempotencyPrefix, cfg.RedisProcessingPrefix)
		if err != nil {
			return nil, fmt.Errorf("initialize redis message deduplicator: %w", err)
		}
		messageDeduplicator = redisMessageDeduplicator
	}

	var replyGenerator chatbot.ReplyGenerator
	if cfg.HasGeminiConfig() {
		replyGenerator = gemini.NewClient(httpClient, cfg.GeminiAPIKey, cfg.GeminiModel, cfg.SystemPrompt)
	} else {
		replyGenerator = fallback.NewGenerator()
	}

	var messageSender chatbot.MessageSender
	if cfg.HasWhatsAppSenderConfig() {
		messageSender = whatsapp.NewClient(httpClient, cfg.WhatsAppAccessToken, cfg.WhatsAppPhoneNumberID)
	} else {
		messageSender = noop.NewSender(logger)
	}

	chatbotService := chatbot.NewService(cfg.AllowedPhoneNumber, replyGenerator, messageSender, conversationRepository, messageArchive, messageDeduplicator)
	handler := httpapi.NewHandler(chatbotService, cfg, logger, metrics)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &Application{server: &http.Server{Addr: cfg.HTTPAddress, Handler: mux, ReadHeaderTimeout: 5 * time.Second}, logger: logger}, nil
}

func (a *Application) Run() error {
	a.logger.Info("server listening", map[string]any{"address": a.server.Addr})
	return a.server.ListenAndServe()
}
