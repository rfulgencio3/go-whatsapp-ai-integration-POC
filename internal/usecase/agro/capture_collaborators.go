package agro

import (
	"context"
	"time"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type ReplyFormatter interface {
	BuildConfirmedReply(event domain.BusinessEvent) string
	BuildHelpReply(topic helpTopic, registered bool) string
	BuildHealthExpenseCorrelationPrompt(event domain.BusinessEvent) string
	BuildCorrelatedExpenseQuestion(state domain.CorrelatedExpenseState) string
	BuildCorrelatedExpenseDeclinedReply() string
	BuildCorrelatedExpenseRecordedReply(state domain.CorrelatedExpenseState) string
	BuildMilkWithdrawalQueryReply(items []domain.MilkWithdrawalAnimal, reference time.Time) string
	BuildRecentHealthTreatmentsReply(items []domain.HealthTreatmentSummary, reference time.Time) string
	BuildMedicineExpenseMonthReply(amount float64, reference time.Time) string
	BuildVetExpenseMonthReply(amount float64, reference time.Time) string
	BuildRecentInputPurchasesReply(items []domain.InputPurchaseSummary, reference time.Time) string
	BuildRejectedReply() string
	BuildUnregisteredNumberReply() string
	BuildAmbiguousContextReply() string
	BuildSingleContextReply(farmName string) string
	BuildAmbiguousContextSelectionReply(options []domain.PhoneContextOption) string
	BuildSelectedContextReply(farmName string) string
	BuildAlreadyRegisteredReply() string
	BuildDraftConfirmationPrompt(event domain.BusinessEvent) string
	BuildDraftConfirmationPromptFromInterpretation(result InterpretationResult) string
	BuildHealthTreatmentQuestion(state domain.HealthTreatmentState) string
}

type WorkflowRouter interface {
	ClassifyConfirmationDecision(text string) confirmationDecision
	ParseContextSelection(text string) int
	IsContextSwitchCommand(text string) bool
	IsOnboardingStartCommand(text string) bool
	IsHelpCommand(text string) bool
	ParseHelpTopic(text string) helpTopic
	IsMilkWithdrawalQuery(text string) bool
	IsRecentTreatmentsQuery(text string) bool
	IsMedicineExpenseMonthQuery(text string) bool
	IsVetExpenseMonthQuery(text string) bool
	IsRecentPurchasesQuery(text string) bool
}

type CapturePersistence interface {
	ProviderOrDefault(provider string) string
	ToDomainMessageType(messageType chat.MessageType) domain.MessageType
	PersistTranscription(ctx context.Context, sourceMessageID string, incomingMessage chat.IncomingMessage, createdAt time.Time) (string, error)
	PersistAssistantMessage(ctx context.Context, conversationID, sourceMessageID string, assistantMessage chat.Message, replyType domain.ReplyType, createdAt time.Time) error
	PersistInterpretation(ctx context.Context, farmID string, sourceMessage domain.SourceMessage, transcriptionID string, occurredAt time.Time) (domain.BusinessEvent, bool, error)
	PersistLegacyConversation(ctx context.Context, phoneNumber string, userMessage, assistantMessage chat.Message) error
	BuildChatMessageFromIncoming(incomingMessage chat.IncomingMessage, text string) chat.Message
}

type HealthTreatmentFlow interface {
	HandleIncomingMessage(ctx context.Context, membership domain.FarmMembership, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error)
}

type CorrelatedExpenseFlow interface {
	HandleIncomingMessage(ctx context.Context, membership domain.FarmMembership, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error)
}

type BusinessQueryFlow interface {
	HandleIncomingMessage(ctx context.Context, membership domain.FarmMembership, message chat.IncomingMessage) (bool, chatbot.ProcessResult, error)
}

type defaultReplyFormatter struct{}

func (defaultReplyFormatter) BuildConfirmedReply(event domain.BusinessEvent) string {
	return buildConfirmedReply(event)
}

func (defaultReplyFormatter) BuildHelpReply(topic helpTopic, registered bool) string {
	return buildHelpReply(topic, registered)
}

func (defaultReplyFormatter) BuildHealthExpenseCorrelationPrompt(event domain.BusinessEvent) string {
	return buildHealthExpenseCorrelationPrompt(event)
}

func (defaultReplyFormatter) BuildCorrelatedExpenseQuestion(state domain.CorrelatedExpenseState) string {
	return buildCorrelatedExpenseQuestion(state)
}

func (defaultReplyFormatter) BuildCorrelatedExpenseDeclinedReply() string {
	return buildCorrelatedExpenseDeclinedReply()
}

func (defaultReplyFormatter) BuildCorrelatedExpenseRecordedReply(state domain.CorrelatedExpenseState) string {
	return buildCorrelatedExpenseRecordedReply(state)
}

func (defaultReplyFormatter) BuildMilkWithdrawalQueryReply(items []domain.MilkWithdrawalAnimal, reference time.Time) string {
	return buildMilkWithdrawalQueryReply(items, reference)
}

func (defaultReplyFormatter) BuildRecentHealthTreatmentsReply(items []domain.HealthTreatmentSummary, reference time.Time) string {
	return buildRecentHealthTreatmentsReply(items, reference)
}

func (defaultReplyFormatter) BuildMedicineExpenseMonthReply(amount float64, reference time.Time) string {
	return buildMedicineExpenseMonthReply(amount, reference)
}

func (defaultReplyFormatter) BuildVetExpenseMonthReply(amount float64, reference time.Time) string {
	return buildVetExpenseMonthReply(amount, reference)
}

func (defaultReplyFormatter) BuildRecentInputPurchasesReply(items []domain.InputPurchaseSummary, reference time.Time) string {
	return buildRecentInputPurchasesReply(items, reference)
}

func (defaultReplyFormatter) BuildRejectedReply() string {
	return buildRejectedReply()
}

func (defaultReplyFormatter) BuildUnregisteredNumberReply() string {
	return buildUnregisteredNumberReply()
}

func (defaultReplyFormatter) BuildAmbiguousContextReply() string {
	return buildAmbiguousContextReply()
}

func (defaultReplyFormatter) BuildSingleContextReply(farmName string) string {
	return buildSingleContextReply(farmName)
}

func (defaultReplyFormatter) BuildAmbiguousContextSelectionReply(options []domain.PhoneContextOption) string {
	return buildAmbiguousContextSelectionReply(options)
}

func (defaultReplyFormatter) BuildSelectedContextReply(farmName string) string {
	return buildSelectedContextReply(farmName)
}

func (defaultReplyFormatter) BuildAlreadyRegisteredReply() string {
	return buildAlreadyRegisteredReply()
}

func (defaultReplyFormatter) BuildDraftConfirmationPrompt(event domain.BusinessEvent) string {
	return buildDraftConfirmationPrompt(event)
}

func (defaultReplyFormatter) BuildDraftConfirmationPromptFromInterpretation(result InterpretationResult) string {
	return buildDraftConfirmationPromptFromInterpretation(result)
}

func (defaultReplyFormatter) BuildHealthTreatmentQuestion(state domain.HealthTreatmentState) string {
	return buildHealthTreatmentQuestion(state)
}

type defaultWorkflowRouter struct{}

func (defaultWorkflowRouter) ClassifyConfirmationDecision(text string) confirmationDecision {
	return classifyConfirmationDecision(text)
}

func (defaultWorkflowRouter) ParseContextSelection(text string) int {
	return parseContextSelection(text)
}

func (defaultWorkflowRouter) IsContextSwitchCommand(text string) bool {
	return isContextSwitchCommand(text)
}

func (defaultWorkflowRouter) IsOnboardingStartCommand(text string) bool {
	return isOnboardingStartCommand(text)
}

func (defaultWorkflowRouter) IsHelpCommand(text string) bool {
	return isHelpCommand(text)
}

func (defaultWorkflowRouter) ParseHelpTopic(text string) helpTopic {
	return parseHelpTopic(text)
}

func (defaultWorkflowRouter) IsMilkWithdrawalQuery(text string) bool {
	return isMilkWithdrawalQuery(text)
}

func (defaultWorkflowRouter) IsRecentTreatmentsQuery(text string) bool {
	return isRecentTreatmentsQuery(text)
}

func (defaultWorkflowRouter) IsMedicineExpenseMonthQuery(text string) bool {
	return isMedicineExpenseMonthQuery(text)
}

func (defaultWorkflowRouter) IsVetExpenseMonthQuery(text string) bool {
	return isVetExpenseMonthQuery(text)
}

func (defaultWorkflowRouter) IsRecentPurchasesQuery(text string) bool {
	return isRecentPurchasesQuery(text)
}

type defaultCapturePersistence struct {
	chatHistory        chatbotConversationRepository
	messageArchive     chatbotMessageArchive
	interpreter        Interpreter
	transcriptions     TranscriptionRepository
	interpretationRuns InterpretationRunRepository
	businessEvents     BusinessEventRepository
	assistantMessages  AssistantMessageRepository
}

type chatbotConversationRepository interface {
	AppendMessage(ctx context.Context, phoneNumber string, message chat.Message) error
}

type chatbotMessageArchive interface {
	RecordMessage(ctx context.Context, phoneNumber string, message chat.Message) error
}

func newDefaultCapturePersistence(
	chatHistory chatbotConversationRepository,
	messageArchive chatbotMessageArchive,
	interpreter Interpreter,
	transcriptions TranscriptionRepository,
	interpretationRuns InterpretationRunRepository,
	businessEvents BusinessEventRepository,
	assistantMessages AssistantMessageRepository,
) CapturePersistence {
	return &defaultCapturePersistence{
		chatHistory:        chatHistory,
		messageArchive:     messageArchive,
		interpreter:        interpreter,
		transcriptions:     transcriptions,
		interpretationRuns: interpretationRuns,
		businessEvents:     businessEvents,
		assistantMessages:  assistantMessages,
	}
}
