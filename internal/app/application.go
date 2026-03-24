package app

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/messaging/noop"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/messaging/whatsapp"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/reply/fallback"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/reply/gemini"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/repository/memory"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/transport/httpapi"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type Application struct {
	server *http.Server
	logger *log.Logger
}

func New(cfg config.Config) *Application {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	httpClient := &http.Client{Timeout: cfg.RequestTimeout}

	conversationRepository := memory.NewConversationRepository(cfg.ConversationHistoryLimit)

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
	}
}

func (a *Application) Run() error {
	a.logger.Printf("server listening on %s", a.server.Addr)
	return a.server.ListenAndServe()
}
