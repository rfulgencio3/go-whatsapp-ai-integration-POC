package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	nooparchive "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/archive/noop"
	postgresarchive "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/archive/postgres"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/messaging/noop"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/messaging/whatsapp"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/reply/fallback"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/reply/gemini"
	memoryrepo "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/repository/memory"
	redisrepo "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/repository/redis"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/transport/httpapi"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type Application struct {
	server *http.Server
	logger *log.Logger
}

func New(cfg config.Config) (*Application, error) {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	httpClient := &http.Client{Timeout: cfg.RequestTimeout}
	startupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conversationRepository := chatbot.ConversationRepository(memoryrepo.NewConversationRepository(cfg.ConversationHistoryLimit))
	if cfg.HasRedisConfig() {
		redisConversationRepository, err := redisrepo.NewConversationRepository(
			startupContext,
			cfg.RedisURL,
			cfg.ConversationHistoryLimit,
			cfg.RedisConversationTTL,
			cfg.RedisKeyPrefix,
		)
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

	chatbotService := chatbot.NewService(
		cfg.AllowedPhoneNumber,
		replyGenerator,
		messageSender,
		conversationRepository,
		messageArchive,
	)

	handler := httpapi.NewHandler(chatbotService, cfg, logger)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &Application{
		server: &http.Server{
			Addr:              cfg.HTTPAddress,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
		logger: logger,
	}, nil
}

func (a *Application) Run() error {
	a.logger.Printf("server listening on %s", a.server.Addr)
	return a.server.ListenAndServe()
}
