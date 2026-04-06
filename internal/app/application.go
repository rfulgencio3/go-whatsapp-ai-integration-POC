package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	nooparchive "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/archive/noop"
	postgresarchive "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/archive/postgres"
	whatsmeowchannel "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/channel/whatsmeow"
	memoryidempotency "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/idempotency/memory"
	redisidempotency "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/idempotency/redis"
	noopinbound "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/inbound/noop"
	twilioinbound "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/inbound/twilio"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/messaging/noop"
	twiliomessaging "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/messaging/twilio"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/messaging/whatsapp"
	memoryqueue "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/queue/memory"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/reply/fallback"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/reply/gemini"
	memoryrepo "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/repository/memory"
	redisrepo "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/repository/redis"
	storagepostgres "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/storage/postgres"
	transcriptionhttpapi "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/transcription/httpapi"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/transport/httpapi"
	agrousecase "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/agro"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
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
	var database *sql.DB

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
		openedDatabase, err := storagepostgres.OpenDatabase(startupContext, cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("initialize postgres database: %w", err)
		}
		database = openedDatabase

		postgresMessageArchive := postgresarchive.NewMessageArchiveWithDatabase(database)
		if err := postgresMessageArchive.EnsureSchema(startupContext); err != nil {
			_ = database.Close()
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

	var transcriptionClient *transcriptionhttpapi.Client
	if cfg.HasTranscriptionConfig() {
		transcriptionClient = transcriptionhttpapi.NewClient(httpClient, cfg.TranscriptionAPIBaseURL)
	}

	var messageSender chatbot.MessageSender
	var startFunc func(context.Context) error
	var stopFunc func()
	var whatsmeowClient *whatsmeowchannel.Client
	if cfg.HasWhatsmeowConfig() {
		client, err := whatsmeowchannel.New(cfg, logger, nil, transcriptionClient)
		if err != nil {
			return nil, fmt.Errorf("initialize whatsmeow adapter: %w", err)
		}
		whatsmeowClient = client
		messageSender = whatsmeowClient
		startFunc = whatsmeowClient.Start
		stopFunc = whatsmeowClient.Stop
	} else if cfg.HasTwilioSenderConfig() {
		messageSender = twiliomessaging.NewClient(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioWhatsAppNumber)
	} else if cfg.HasWhatsAppSenderConfig() {
		messageSender = whatsapp.NewClient(httpClient, cfg.WhatsAppAccessToken, cfg.WhatsAppPhoneNumberID)
	} else {
		messageSender = noop.NewSender(logger)
	}

	incomingPreprocessor := chatbot.IncomingMessagePreprocessor(noopinbound.NewPreprocessor())
	if cfg.HasTwilioWebhookConfig() {
		incomingPreprocessor = twilioinbound.NewPreprocessor(httpClient, cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TranscriptionMaxBytes, transcriptionClient)
	}

	interpreter := agrousecase.NewRuleBasedInterpreter()
	replyOverrideResolver := agrousecase.NewReplyOverrideResolver(interpreter)
	chatbotService := chatbot.NewService(cfg.AllowedPhoneNumber, replyGenerator, replyOverrideResolver, messageSender, incomingPreprocessor, conversationRepository, messageArchive, messageDeduplicator)
	messageProcessor := chatbot.MessageProcessor(chatbotService)
	if database != nil {
		messageProcessor = agrousecase.NewCaptureService(
			logger,
			chatbotService,
			messageSender,
			conversationRepository,
			messageArchive,
			interpreter,
			storagepostgres.NewFarmMembershipRepository(database),
			storagepostgres.NewFarmRegistrationRepository(database),
			storagepostgres.NewPhoneContextStateRepository(database),
			storagepostgres.NewOnboardingStateRepository(database),
			storagepostgres.NewConversationRepository(database),
			storagepostgres.NewSourceMessageRepository(database),
			storagepostgres.NewTranscriptionRepository(database),
			storagepostgres.NewInterpretationRunRepository(database),
			storagepostgres.NewBusinessEventRepository(database),
			storagepostgres.NewAssistantMessageRepository(database),
			storagepostgres.NewOnboardingMessageRepository(database),
		)
	}
	if whatsmeowClient != nil {
		whatsmeowClient.SetProcessor(messageProcessor)
	}
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
		startFunc: startFunc,
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
