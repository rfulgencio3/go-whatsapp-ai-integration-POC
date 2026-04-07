package app

import (
	"context"
	"net/http"
	"time"

	memoryqueue "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/queue/memory"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/transport/httpapi"
)

type Application struct {
	server    *http.Server
	logger    *observability.Logger
	startFunc func(context.Context) error
	stopFunc  func()
}

func New(cfg config.Config) (*Application, error) {
	logger := observability.NewLogger()
	metrics := observability.NewMetrics()
	httpClient := &http.Client{Timeout: cfg.RequestTimeout}
	startupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conversationRepository, err := buildConversationRepository(startupContext, cfg)
	if err != nil {
		return nil, err
	}
	messageArchive, database, err := buildMessageArchive(startupContext, cfg)
	if err != nil {
		return nil, err
	}
	messageDeduplicator, err := buildMessageDeduplicator(startupContext, cfg)
	if err != nil {
		return nil, err
	}
	replyGenerator := buildReplyGenerator(cfg, httpClient)
	transcriptionClient := buildTranscriptionClient(cfg, httpClient)
	messaging, err := buildMessagingRuntime(cfg, logger, httpClient, transcriptionClient)
	if err != nil {
		return nil, err
	}
	incomingPreprocessor := buildIncomingPreprocessor(cfg, httpClient, transcriptionClient)
	chatbotService, interpreter := buildChatbotService(cfg, messaging.sender, conversationRepository, messageArchive, replyGenerator, incomingPreprocessor, messageDeduplicator)
	messageProcessor := buildMessageProcessor(logger, database, messaging.sender, conversationRepository, messageArchive, chatbotService, interpreter)
	if messaging.whatsmeowClient != nil {
		messaging.whatsmeowClient.SetProcessor(messageProcessor)
	}
	stopFunc := messaging.stopFunc
	if database != nil {
		stopFunc = composeStopFunc(stopFunc, func() {
			_ = database.Close()
		})
	}
	messageQueue := memoryqueue.NewQueue(cfg.WebhookQueueWorkers, cfg.WebhookQueueBufferSize, cfg.WebhookQueueMaxRetries, cfg.WebhookQueueRetryDelay, messageProcessor, logger, metrics)
	handler := httpapi.NewHandler(chatbotService, messageQueue, cfg, logger, metrics)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return &Application{
		server:    &http.Server{Addr: cfg.HTTPAddress, Handler: mux, ReadHeaderTimeout: 5 * time.Second},
		logger:    logger,
		startFunc: messaging.startFunc,
		stopFunc:  stopFunc,
	}, nil
}

func composeStopFunc(current func(), next func()) func() {
	switch {
	case current == nil:
		return next
	case next == nil:
		return current
	default:
		return func() {
			current()
			next()
		}
	}
}

func (a *Application) Run() error {
	if a.startFunc != nil {
		startupContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.startFunc(startupContext); err != nil {
			return err
		}
	}
	defer func() {
		if a.stopFunc != nil {
			a.stopFunc()
		}
	}()

	a.logger.Info("server listening", map[string]any{"address": a.server.Addr})
	return a.server.ListenAndServe()
}
