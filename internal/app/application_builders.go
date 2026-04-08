package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

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
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/reply/fallback"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/reply/gemini"
	memoryrepo "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/repository/memory"
	redisrepo "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/repository/redis"
	storagepostgres "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/storage/postgres"
	transcriptionhttpapi "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/transcription/httpapi"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	agrousecase "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/agro"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type messagingRuntime struct {
	sender          chatbot.MessageSender
	startFunc       func(context.Context) error
	stopFunc        func()
	whatsmeowClient *whatsmeowchannel.Client
}

func buildConversationRepository(ctx context.Context, cfg config.Config) (chatbot.ConversationRepository, error) {
	repository := chatbot.ConversationRepository(memoryrepo.NewConversationRepository(cfg.ConversationHistoryLimit))
	if !cfg.HasRedisConfig() {
		return repository, nil
	}

	redisConversationRepository, err := redisrepo.NewConversationRepository(ctx, cfg.RedisURL, cfg.ConversationHistoryLimit, cfg.RedisConversationTTL, cfg.RedisKeyPrefix)
	if err != nil {
		return nil, fmt.Errorf("initialize redis conversation repository: %w", err)
	}

	return redisConversationRepository, nil
}

func buildMessageArchive(ctx context.Context, cfg config.Config) (chatbot.MessageArchive, *sql.DB, error) {
	archive := chatbot.MessageArchive(nooparchive.NewMessageArchive())
	if !cfg.HasDatabaseConfig() {
		return archive, nil, nil
	}

	database, err := storagepostgres.OpenDatabase(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize postgres database: %w", err)
	}

	postgresMessageArchive := postgresarchive.NewMessageArchiveWithDatabase(database)
	if err := postgresMessageArchive.EnsureSchema(ctx); err != nil {
		_ = database.Close()
		return nil, nil, fmt.Errorf("initialize postgres message archive: %w", err)
	}

	return postgresMessageArchive, database, nil
}

func buildMessageDeduplicator(ctx context.Context, cfg config.Config) (chatbot.MessageDeduplicator, error) {
	store := chatbot.MessageDeduplicator(memoryidempotency.NewStore(cfg.WebhookIdempotencyTTL, cfg.WebhookProcessingTTL))
	if !cfg.HasRedisConfig() {
		return store, nil
	}

	redisStore, err := redisidempotency.NewStore(ctx, cfg.RedisURL, cfg.WebhookIdempotencyTTL, cfg.WebhookProcessingTTL, cfg.RedisIdempotencyPrefix, cfg.RedisProcessingPrefix)
	if err != nil {
		return nil, fmt.Errorf("initialize redis message deduplicator: %w", err)
	}

	return redisStore, nil
}

func buildReplyGenerator(cfg config.Config, httpClient *http.Client) chatbot.ReplyGenerator {
	if cfg.HasGeminiConfig() {
		return gemini.NewClient(httpClient, cfg.GeminiAPIKey, cfg.GeminiModel, cfg.SystemPrompt)
	}

	return fallback.NewGenerator()
}

func buildTranscriptionClient(cfg config.Config, httpClient *http.Client) *transcriptionhttpapi.Client {
	if !cfg.HasTranscriptionConfig() {
		return nil
	}

	return transcriptionhttpapi.NewClient(httpClient, cfg.TranscriptionAPIBaseURL)
}

func buildMessagingRuntime(cfg config.Config, logger *observability.Logger, httpClient *http.Client, transcriptionClient *transcriptionhttpapi.Client) (messagingRuntime, error) {
	switch {
	case cfg.HasWhatsmeowConfig():
		client, err := whatsmeowchannel.New(cfg, logger, nil, transcriptionClient)
		if err != nil {
			return messagingRuntime{}, fmt.Errorf("initialize whatsmeow adapter: %w", err)
		}
		return messagingRuntime{
			sender:          client,
			startFunc:       client.Start,
			stopFunc:        client.Stop,
			whatsmeowClient: client,
		}, nil
	case cfg.HasTwilioSenderConfig():
		return messagingRuntime{sender: twiliomessaging.NewClient(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioWhatsAppNumber)}, nil
	case cfg.HasWhatsAppSenderConfig():
		return messagingRuntime{sender: whatsapp.NewClient(httpClient, cfg.WhatsAppAccessToken, cfg.WhatsAppPhoneNumberID)}, nil
	default:
		return messagingRuntime{sender: noop.NewSender(logger)}, nil
	}
}

func buildIncomingPreprocessor(cfg config.Config, httpClient *http.Client, transcriptionClient *transcriptionhttpapi.Client) chatbot.IncomingMessagePreprocessor {
	if cfg.HasTwilioWebhookConfig() {
		return twilioinbound.NewPreprocessor(httpClient, cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TranscriptionMaxBytes, transcriptionClient)
	}

	return noopinbound.NewPreprocessor()
}

func buildChatbotService(
	cfg config.Config,
	messageSender chatbot.MessageSender,
	conversationRepository chatbot.ConversationRepository,
	messageArchive chatbot.MessageArchive,
	replyGenerator chatbot.ReplyGenerator,
	incomingPreprocessor chatbot.IncomingMessagePreprocessor,
	messageDeduplicator chatbot.MessageDeduplicator,
) (*chatbot.Service, agrousecase.Interpreter) {
	interpreter := agrousecase.NewRuleBasedInterpreter()
	replyOverrideResolver := agrousecase.NewReplyOverrideResolver(interpreter)
	chatbotService := chatbot.NewService(cfg.AllowedPhoneNumber, replyGenerator, replyOverrideResolver, messageSender, incomingPreprocessor, conversationRepository, messageArchive, messageDeduplicator)
	return chatbotService, interpreter
}

func buildMessageProcessor(
	logger *observability.Logger,
	database *sql.DB,
	messageSender chatbot.MessageSender,
	conversationRepository chatbot.ConversationRepository,
	messageArchive chatbot.MessageArchive,
	chatbotService *chatbot.Service,
	interpreter agrousecase.Interpreter,
) chatbot.MessageProcessor {
	messageProcessor := chatbot.MessageProcessor(chatbotService)
	if database == nil {
		return messageProcessor
	}

	captureService := agrousecase.NewCaptureService(
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
	captureService.SetHealthTreatmentStateRepository(storagepostgres.NewHealthTreatmentStateRepository(database))
	captureService.SetCorrelatedExpenseStateRepository(storagepostgres.NewCorrelatedExpenseStateRepository(database))
	captureService.SetFarmAnimalRepository(storagepostgres.NewFarmAnimalRepository(database))
	captureService.EnableBusinessQueryFlow()
	return captureService
}
