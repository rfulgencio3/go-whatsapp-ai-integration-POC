package agro

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type CaptureService struct {
	logger             *observability.Logger
	downstream         chatbot.MessageProcessor
	messageSender      chatbot.MessageSender
	chatHistory        chatbot.ConversationRepository
	messageArchive     chatbot.MessageArchive
	interpreter        Interpreter
	farmMemberships    FarmMembershipRepository
	farmRegistrations  FarmRegistrationRepository
	phoneContexts      PhoneContextStateRepository
	onboardingStates   OnboardingStateRepository
	onboardingMessages OnboardingMessageRepository
	conversations      ConversationRepository
	sourceMessages     SourceMessageRepository
	transcriptions     TranscriptionRepository
	interpretationRuns InterpretationRunRepository
	businessEvents     BusinessEventRepository
	assistantMessages  AssistantMessageRepository
}

type membershipResolution string

const (
	membershipResolutionUnavailable membershipResolution = "unavailable"
	membershipResolutionNotFound    membershipResolution = "not_found"
	membershipResolutionAmbiguous   membershipResolution = "ambiguous"
	membershipResolutionResolved    membershipResolution = "resolved"
	membershipResolutionSelected    membershipResolution = "selected"
)

func NewCaptureService(
	logger *observability.Logger,
	downstream chatbot.MessageProcessor,
	messageSender chatbot.MessageSender,
	chatHistory chatbot.ConversationRepository,
	messageArchive chatbot.MessageArchive,
	interpreter Interpreter,
	farmMemberships FarmMembershipRepository,
	farmRegistrations FarmRegistrationRepository,
	phoneContexts PhoneContextStateRepository,
	onboardingStates OnboardingStateRepository,
	conversations ConversationRepository,
	sourceMessages SourceMessageRepository,
	transcriptions TranscriptionRepository,
	interpretationRuns InterpretationRunRepository,
	businessEvents BusinessEventRepository,
	assistantMessages AssistantMessageRepository,
	onboardingMessages ...OnboardingMessageRepository,
) *CaptureService {
	if logger == nil {
		logger = observability.NewLogger()
	}

	var onboardingMessageRepository OnboardingMessageRepository
	if len(onboardingMessages) > 0 {
		onboardingMessageRepository = onboardingMessages[0]
	}

	return &CaptureService{
		logger:             logger,
		downstream:         downstream,
		messageSender:      messageSender,
		chatHistory:        chatHistory,
		messageArchive:     messageArchive,
		interpreter:        interpreter,
		farmMemberships:    farmMemberships,
		farmRegistrations:  farmRegistrations,
		phoneContexts:      phoneContexts,
		onboardingStates:   onboardingStates,
		onboardingMessages: onboardingMessageRepository,
		conversations:      conversations,
		sourceMessages:     sourceMessages,
		transcriptions:     transcriptions,
		interpretationRuns: interpretationRuns,
		businessEvents:     businessEvents,
		assistantMessages:  assistantMessages,
	}
}

func (s *CaptureService) ProcessIncomingMessage(ctx context.Context, message chat.IncomingMessage) (chatbot.ProcessResult, error) {
	if s.downstream == nil {
		return chatbot.ProcessResult{}, nil
	}
	handled, result, err := s.handleOnboarding(ctx, message)
	if err != nil {
		return chatbot.ProcessResult{}, err
	}
	if handled {
		return result, nil
	}
	handled, result, err = s.handleContextSwitchRequest(ctx, message)
	if err != nil {
		return chatbot.ProcessResult{}, err
	}
	if handled {
		return result, nil
	}

	membership, resolution, err := s.resolveMembership(ctx, message.PhoneNumber)
	if err != nil {
		return chatbot.ProcessResult{}, err
	}
	resolved := resolution == membershipResolutionResolved
	if resolved {
		handled, result, err := s.handleConfirmationMessage(ctx, membership, message)
		if err != nil {
			return chatbot.ProcessResult{}, err
		}
		if handled {
			return result, nil
		}
	}
	if !resolved {
		handled, result, err := s.handleUnresolvedMembership(ctx, resolution, message)
		if err != nil {
			return chatbot.ProcessResult{}, err
		}
		if handled {
			return result, nil
		}
	}

	result, err = s.downstream.ProcessIncomingMessage(ctx, message)
	if err != nil || result.Duplicate {
		return result, err
	}

	if err := s.captureProcessedInteraction(ctx, membership, resolved, result); err != nil {
		s.logger.Error("agro inbound capture failed", map[string]any{
			"phone_number": domain.NormalizePhoneNumber(result.PhoneNumber),
			"message_id":   strings.TrimSpace(result.IncomingMessage.MessageID),
			"provider":     strings.TrimSpace(result.IncomingMessage.Provider),
			"error":        err.Error(),
		})
	}

	return result, nil
}

func providerOrDefault(provider string) string {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "whatsapp"
	}

	return provider
}

func toDomainMessageType(messageType chat.MessageType) domain.MessageType {
	switch messageType {
	case chat.MessageTypeText:
		return domain.MessageTypeText
	case chat.MessageTypeAudio:
		return domain.MessageTypeAudio
	case chat.MessageTypeImage:
		return domain.MessageTypeImage
	case chat.MessageTypeDocument:
		return domain.MessageTypeDocument
	default:
		return domain.MessageTypeUnsupported
	}
}

func (s *CaptureService) persistTranscription(ctx context.Context, sourceMessageID string, incomingMessage chat.IncomingMessage, createdAt time.Time) (string, error) {
	if s.transcriptions == nil || strings.TrimSpace(incomingMessage.TranscriptionID) == "" {
		return "", nil
	}

	transcription := domain.Transcription{
		ID:              uuid.NewString(),
		SourceMessageID: sourceMessageID,
		Provider:        "transcription-api",
		ProviderRef:     strings.TrimSpace(incomingMessage.TranscriptionID),
		TranscriptText:  strings.TrimSpace(incomingMessage.Text),
		Language:        strings.TrimSpace(incomingMessage.TranscriptionLanguage),
		DurationSeconds: incomingMessage.AudioDurationSeconds,
		CreatedAt:       createdAt,
	}

	if err := s.transcriptions.Create(ctx, &transcription); err != nil {
		return "", err
	}

	return transcription.ID, nil
}

func (s *CaptureService) persistAssistantMessage(ctx context.Context, conversationID, sourceMessageID string, assistantMessage chat.Message, replyType domain.ReplyType, createdAt time.Time) error {
	if s.assistantMessages == nil || strings.TrimSpace(assistantMessage.Text) == "" {
		return nil
	}
	if replyType == "" {
		replyType = domain.ReplyTypeText
	}

	message := domain.AssistantMessage{
		ID:                uuid.NewString(),
		ConversationID:    conversationID,
		SourceMessageID:   sourceMessageID,
		Provider:          providerOrDefault(assistantMessage.Provider),
		ProviderMessageID: strings.TrimSpace(assistantMessage.ProviderMessageID),
		ReplyType:         replyType,
		Body:              strings.TrimSpace(assistantMessage.Text),
		CreatedAt:         createdAt,
	}

	return s.assistantMessages.Create(ctx, &message)
}

func (s *CaptureService) persistInterpretation(ctx context.Context, farmID string, sourceMessage domain.SourceMessage, transcriptionID string, occurredAt time.Time) (domain.BusinessEvent, bool, error) {
	if s.interpreter == nil || s.interpretationRuns == nil {
		return domain.BusinessEvent{}, false, nil
	}

	interpretation, err := s.interpreter.Interpret(ctx, InterpretationInput{
		MessageType: sourceMessage.MessageType,
		Text:        sourceMessage.RawText,
		OccurredAt:  occurredAt,
	})
	if err != nil {
		return domain.BusinessEvent{}, false, err
	}
	if strings.TrimSpace(interpretation.NormalizedIntent) == "" {
		return domain.BusinessEvent{}, false, nil
	}

	run := domain.InterpretationRun{
		ID:                   uuid.NewString(),
		SourceMessageID:      sourceMessage.ID,
		TranscriptionID:      strings.TrimSpace(transcriptionID),
		ModelProvider:        interpreterProvider,
		ModelName:            interpreterModel,
		PromptVersion:        promptVersion,
		NormalizedIntent:     interpretation.NormalizedIntent,
		Confidence:           interpretation.Confidence,
		RequiresConfirmation: interpretation.RequiresConfirmation,
		RawOutputJSON:        interpretation.RawOutputJSON,
		CreatedAt:            occurredAt,
	}
	if err := s.interpretationRuns.Create(ctx, &run); err != nil {
		return domain.BusinessEvent{}, false, err
	}
	if s.businessEvents == nil {
		return domain.BusinessEvent{}, false, nil
	}

	event := domain.BusinessEvent{
		ID:                  uuid.NewString(),
		FarmID:              farmID,
		SourceMessageID:     sourceMessage.ID,
		InterpretationRunID: run.ID,
		Category:            interpretation.Category,
		Subcategory:         interpretation.Subcategory,
		OccurredAt:          interpretation.OccurredAt,
		Description:         interpretation.Description,
		Amount:              interpretation.Amount,
		Currency:            interpretation.Currency,
		Quantity:            interpretation.Quantity,
		Unit:                interpretation.Unit,
		AnimalCode:          strings.TrimSpace(interpretation.AnimalCode),
		Status:              domain.EventStatusDraft,
		ConfirmedByUser:     false,
		CreatedAt:           occurredAt,
		UpdatedAt:           occurredAt,
	}

	if err := s.businessEvents.Create(ctx, &event); err != nil {
		return domain.BusinessEvent{}, false, err
	}
	if err := s.businessEvents.CreateAttributes(ctx, event.ID, interpretation.Attributes); err != nil {
		return domain.BusinessEvent{}, false, err
	}

	return event, interpretation.RequiresConfirmation, nil
}

func (s *CaptureService) persistLegacyConversation(ctx context.Context, phoneNumber string, userMessage, assistantMessage chat.Message) error {
	if s.chatHistory == nil || s.messageArchive == nil {
		return nil
	}
	if err := s.chatHistory.AppendMessage(ctx, phoneNumber, userMessage); err != nil {
		return err
	}
	if err := s.messageArchive.RecordMessage(ctx, phoneNumber, userMessage); err != nil {
		return err
	}
	if err := s.chatHistory.AppendMessage(ctx, phoneNumber, assistantMessage); err != nil {
		return err
	}
	if err := s.messageArchive.RecordMessage(ctx, phoneNumber, assistantMessage); err != nil {
		return err
	}

	return nil
}

type confirmationDecision string

const (
	confirmationAccepted confirmationDecision = "accepted"
	confirmationRejected confirmationDecision = "rejected"
)

func classifyConfirmationDecision(text string) confirmationDecision {
	normalized := normalizeText(text)
	switch normalized {
	case "sim", "s", "ok", "confirmar", "confirmado", "pode confirmar", "isso":
		return confirmationAccepted
	case "nao", "não", "n", "cancelar", "corrigir", "errado":
		return confirmationRejected
	default:
		return ""
	}
}

func buildConfirmedReply(event domain.BusinessEvent) string {
	switch {
	case event.Category == "finance" && event.Subcategory == "input_purchase" && event.Amount != nil && event.Quantity != nil && strings.TrimSpace(event.Unit) != "":
		return fmt.Sprintf("Registro confirmado: compra de insumos de R$ %.2f, %.3g %s.", *event.Amount, *event.Quantity, event.Unit)
	case event.Category == "finance" && event.Subcategory == "expense" && event.Amount != nil:
		return fmt.Sprintf("Registro confirmado: despesa de R$ %.2f.", *event.Amount)
	case event.Category == "finance" && event.Subcategory == "revenue" && event.Amount != nil:
		return fmt.Sprintf("Registro confirmado: receita de R$ %.2f.", *event.Amount)
	case event.Category == "reproduction" && event.Subcategory == "insemination":
		return "Registro confirmado: evento de inseminacao salvo."
	default:
		return "Registro confirmado com sucesso."
	}
}

func buildRejectedReply() string {
	return "Entendi. Nao vou considerar esse registro. Envie a correcao em uma unica mensagem."
}

func buildUnregisteredNumberReply() string {
	return "Seu numero ainda nao esta vinculado a uma fazenda. Peça o cadastro do seu telefone para continuar."
}

func buildAmbiguousContextReply() string {
	return "Seu numero esta vinculado a mais de uma fazenda. Ajuste o cadastro antes de continuar."
}

func buildSingleContextReply(farmName string) string {
	return fmt.Sprintf("Seu numero ja esta vinculado a %s.", fallbackFarmName(domain.PhoneContextOption{FarmName: farmName}, 1))
}

func buildAmbiguousContextSelectionReply(options []domain.PhoneContextOption) string {
	var builder strings.Builder
	builder.WriteString("Seu numero esta vinculado a mais de uma fazenda. Responda com o numero:\n")
	for index, option := range options {
		builder.WriteString(fmt.Sprintf("%d. %s", index+1, fallbackFarmName(option, index+1)))
		if index < len(options)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func buildSelectedContextReply(farmName string) string {
	return fmt.Sprintf("Contexto definido para %s. Envie a informacao novamente.", fallbackFarmName(domain.PhoneContextOption{FarmName: farmName}, 1))
}

func parseContextSelection(text string) int {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return 0
	}
	if len(normalized) != 1 || normalized[0] < '1' || normalized[0] > '9' {
		return 0
	}

	return int(normalized[0] - '0')
}

func isContextSwitchCommand(text string) bool {
	switch normalizeText(text) {
	case "trocar fazenda", "mudar fazenda", "alternar fazenda", "selecionar fazenda", "trocar contexto", "mudar contexto":
		return true
	default:
		return false
	}
}

func isOnboardingStartCommand(text string) bool {
	switch normalizeText(text) {
	case "cadastrar", "cadastro", "quero cadastrar", "iniciar cadastro", "me cadastrar":
		return true
	default:
		return false
	}
}

func buildAlreadyRegisteredReply() string {
	return "Seu numero ja esta cadastrado. Pode enviar seus registros normalmente."
}

func fallbackFarmName(option domain.PhoneContextOption, position int) string {
	if strings.TrimSpace(option.FarmName) != "" {
		return strings.TrimSpace(option.FarmName)
	}

	return fmt.Sprintf("Fazenda %d", position)
}

func buildDraftConfirmationPrompt(event domain.BusinessEvent) string {
	return buildDraftConfirmationPromptFromInterpretation(InterpretationResult{
		Category:    event.Category,
		Subcategory: event.Subcategory,
		Description: event.Description,
		AnimalCode:  event.AnimalCode,
		Amount:      event.Amount,
		Currency:    event.Currency,
		Quantity:    event.Quantity,
		Unit:        event.Unit,
		OccurredAt:  event.OccurredAt,
	})
}

func buildDraftConfirmationPromptFromInterpretation(result InterpretationResult) string {
	lines := []string{
		fmt.Sprintf("Categoria: %s", humanCategoryLabel(result.Category, result.Subcategory)),
	}

	if detail := buildConfirmationDetail(result); detail != "" {
		lines = append(lines, detail)
	}
	if result.Amount != nil {
		lines = append(lines, fmt.Sprintf("Valor: %s", formatCurrency(result.Amount, result.Currency)))
	}
	if result.Quantity != nil {
		lines = append(lines, fmt.Sprintf("Quantidade: %s", formatQuantity(result.Quantity, result.Unit)))
	}
	if occurredAt := formatOccurredAt(result.OccurredAt); occurredAt != "" {
		lines = append(lines, fmt.Sprintf("Data: %s", occurredAt))
	}
	lines = append(lines, "Responda SIM para confirmar ou NAO para corrigir.")

	return strings.Join(lines, "\n")
}

func humanCategoryLabel(category, subcategory string) string {
	switch {
	case category == "health" && subcategory == "mastitis_treatment":
		return "Saude animal"
	case category == "health" && subcategory == "hoof_treatment":
		return "Saude animal"
	case category == "health" && subcategory == "bloat":
		return "Saude animal"
	case category == "finance" && subcategory == "input_purchase":
		return "Compra de insumos"
	case category == "finance" && subcategory == "expense":
		return "Despesa"
	case category == "finance" && subcategory == "revenue":
		return "Receita"
	case category == "reproduction" && subcategory == "insemination":
		return "Manejo reprodutivo"
	case category == "operations" && subcategory == "note":
		return "Observacao operacional"
	default:
		return "Registro operacional"
	}
}

func buildConfirmationDetail(result InterpretationResult) string {
	description := strings.TrimSpace(result.Description)
	switch {
	case result.Category == "health":
		return buildHealthConfirmationDetail(result, description)
	case result.Category == "finance" && result.Subcategory == "input_purchase" && description != "":
		return fmt.Sprintf("Descricao: %s", description)
	case result.Category == "finance" && result.Subcategory == "expense" && description != "":
		return fmt.Sprintf("Descricao: %s", description)
	case result.Category == "finance" && result.Subcategory == "revenue" && description != "":
		return fmt.Sprintf("Descricao: %s", description)
	case result.Category == "reproduction" && result.Subcategory == "insemination":
		if description == "" {
			return "Evento: inseminacao"
		}
		return fmt.Sprintf("Evento: %s", description)
	case description != "":
		return fmt.Sprintf("Descricao: %s", description)
	default:
		return ""
	}
}

func buildHealthConfirmationDetail(result InterpretationResult, description string) string {
	lines := make([]string, 0, 4)
	if strings.TrimSpace(result.AnimalCode) != "" {
		lines = append(lines, fmt.Sprintf("Animal: %s", result.AnimalCode))
	}
	switch result.Subcategory {
	case "mastitis_treatment":
		lines = append(lines, "Problema: teta/mastite")
	case "hoof_treatment":
		lines = append(lines, "Problema: casco/manqueira")
	case "bloat":
		lines = append(lines, "Problema: gases/timpanismo")
	}
	if result.Attributes != nil {
		if teats := strings.TrimSpace(result.Attributes["affected_teats"]); teats != "" {
			lines = append(lines, fmt.Sprintf("Tetas afetadas: %s", teats))
		}
		if strings.EqualFold(strings.TrimSpace(result.Attributes["milk_withdrawal"]), "true") {
			lines = append(lines, "Restricao: nao tirar leite")
		}
	}
	if description != "" {
		lines = append(lines, fmt.Sprintf("Descricao: %s", description))
	}
	return strings.Join(lines, "\n")
}

func formatCurrency(amount *float64, currency string) string {
	if amount == nil {
		return ""
	}
	if strings.TrimSpace(currency) == "" || strings.EqualFold(currency, "BRL") {
		return fmt.Sprintf("R$ %.2f", *amount)
	}
	return fmt.Sprintf("%s %.2f", strings.ToUpper(strings.TrimSpace(currency)), *amount)
}

func formatQuantity(quantity *float64, unit string) string {
	if quantity == nil {
		return ""
	}
	if strings.TrimSpace(unit) == "" {
		return fmt.Sprintf("%.3g", *quantity)
	}
	return fmt.Sprintf("%.3g %s", *quantity, unit)
}

func formatOccurredAt(occurredAt *time.Time) string {
	if occurredAt == nil {
		return ""
	}
	return occurredAt.In(time.FixedZone("BRT", -3*60*60)).Format("02/01/2006 15:04")
}

func buildChatMessageFromIncoming(incomingMessage chat.IncomingMessage, text string) chat.Message {
	message := chat.Message{
		Role:                  chat.UserRole,
		Text:                  text,
		CreatedAt:             time.Now().UTC(),
		Type:                  incomingMessage.Type,
		Provider:              strings.TrimSpace(incomingMessage.Provider),
		ProviderMessageID:     strings.TrimSpace(incomingMessage.MessageID),
		TranscriptionID:       strings.TrimSpace(incomingMessage.TranscriptionID),
		TranscriptionLanguage: strings.TrimSpace(incomingMessage.TranscriptionLanguage),
		AudioDurationSeconds:  incomingMessage.AudioDurationSeconds,
	}
	if message.Type == "" {
		message.Type = chat.MessageTypeText
	}
	if len(incomingMessage.MediaAttachments) > 0 {
		message.MediaURL = strings.TrimSpace(incomingMessage.MediaAttachments[0].URL)
		message.MediaContentType = strings.TrimSpace(incomingMessage.MediaAttachments[0].ContentType)
		message.MediaFilename = strings.TrimSpace(incomingMessage.MediaAttachments[0].Filename)
	}
	return message
}
