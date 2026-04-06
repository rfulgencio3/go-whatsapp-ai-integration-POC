package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type Handler struct {
	chatbotService *chatbot.Service
	messageQueue   chatbot.MessageQueue
	config         config.Config
	logger         *observability.Logger
	metrics        *observability.Metrics
}

func NewHandler(chatbotService *chatbot.Service, messageQueue chatbot.MessageQueue, cfg config.Config, logger *observability.Logger, metrics *observability.Metrics) *Handler {
	if metrics == nil {
		metrics = observability.NewMetrics()
	}

	return &Handler{chatbotService: chatbotService, messageQueue: messageQueue, config: cfg, logger: logger, metrics: metrics}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", h.handleRoot)
	mux.HandleFunc("/privacy-policy", h.handlePrivacyPolicy)
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/metrics", h.handleMetrics)
	mux.HandleFunc("/webhook", h.handleWebhook)
	mux.HandleFunc("/webhook/twilio", h.handleTwilioWebhook)
	mux.HandleFunc("/simulate", h.handleSimulation)
	mux.HandleFunc("/swagger", h.handleSwaggerUI)
	mux.HandleFunc("/swagger/", h.handleSwaggerRoute)
}

func (h *Handler) handleRoot(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(responseWriter, request)
		return
	}

	http.Redirect(responseWriter, request, "/swagger", http.StatusTemporaryRedirect)
}

func (h *Handler) handleHealth(responseWriter http.ResponseWriter, _ *http.Request) {
	writeJSON(responseWriter, http.StatusOK, HealthResponse{
		Status:                    "ok",
		WhatsAppSenderConfigured:  h.config.HasWhatsAppSenderConfig(),
		WhatsAppWebhookConfigured: h.config.HasWhatsAppWebhookConfig(),
		TwilioConfigured:          h.config.HasTwilioSenderConfig() || h.config.HasTwilioWebhookConfig(),
		TranscriptionConfigured:   h.config.HasTranscriptionConfig(),
		MessagingProvider:         h.config.MessagingProvider(),
		GeminiConfigured:          h.config.HasGeminiConfig(),
		GeminiModel:               h.config.GeminiModel,
	})
}

func (h *Handler) handleMetrics(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(responseWriter, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(responseWriter, http.StatusOK, h.metrics.Snapshot())
}

func (h *Handler) handleWebhook(responseWriter http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		h.handleWebhookVerification(responseWriter, request)
	case http.MethodPost:
		h.handleWebhookNotification(responseWriter, request)
	default:
		writeError(responseWriter, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleTwilioWebhook(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(responseWriter, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	h.handleTwilioWebhookNotification(responseWriter, request)
}

func (h *Handler) handleWebhookVerification(responseWriter http.ResponseWriter, request *http.Request) {
	mode := request.URL.Query().Get("hub.mode")
	verifyToken := request.URL.Query().Get("hub.verify_token")
	challenge := request.URL.Query().Get("hub.challenge")

	if mode != "subscribe" || verifyToken == "" || !h.config.HasWhatsAppWebhookConfig() || verifyToken != h.config.WhatsAppVerifyToken {
		writeError(responseWriter, http.StatusForbidden, "forbidden")
		return
	}

	responseWriter.WriteHeader(http.StatusOK)
	_, _ = responseWriter.Write([]byte(challenge))
}

func (h *Handler) handleWebhookNotification(responseWriter http.ResponseWriter, request *http.Request) {
	h.metrics.IncWebhookRequests()

	body, err := io.ReadAll(request.Body)
	if err != nil {
		h.metrics.IncWebhookFailures()
		writeError(responseWriter, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateWebhookSignature(request.Header.Get("X-Hub-Signature-256"), body, h.config.WhatsAppAppSecret); err != nil {
		h.metrics.IncWebhookFailures()
		writeError(responseWriter, http.StatusUnauthorized, err.Error())
		return
	}

	var notification WhatsAppWebhookNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		h.metrics.IncWebhookFailures()
		writeError(responseWriter, http.StatusBadRequest, "invalid json")
		return
	}

	messages := notification.ExtractIncomingMessages()
	h.metrics.AddWebhookMessages(len(messages))

	for _, message := range messages {
		if err := h.messageQueue.Enqueue(request.Context(), message); err != nil {
			h.metrics.IncWebhookEnqueueFailure()
			h.logger.Error("enqueue webhook message failed", map[string]any{"phone_number": message.PhoneNumber, "message_id": message.MessageID, "error": err.Error()})
			writeError(responseWriter, http.StatusInternalServerError, "failed to enqueue webhook message")
			return
		}
	}

	writeJSON(responseWriter, http.StatusOK, WebhookResponse{Status: "received"})
}

func (h *Handler) handleTwilioWebhookNotification(responseWriter http.ResponseWriter, request *http.Request) {
	h.metrics.IncWebhookRequests()

	body, err := io.ReadAll(request.Body)
	if err != nil {
		h.metrics.IncWebhookFailures()
		writeError(responseWriter, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateTwilioWebhookSignature(request.Header.Get("X-Twilio-Signature"), h.resolveTwilioWebhookURL(request), body, h.config.TwilioAuthToken); err != nil {
		h.metrics.IncWebhookFailures()
		writeError(responseWriter, http.StatusUnauthorized, err.Error())
		return
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		h.metrics.IncWebhookFailures()
		writeError(responseWriter, http.StatusBadRequest, "invalid form payload")
		return
	}

	message := extractIncomingTwilioMessage(values)
	if message.PhoneNumber == "" {
		h.metrics.IncWebhookFailures()
		writeError(responseWriter, http.StatusBadRequest, "phone number is required")
		return
	}

	h.metrics.AddWebhookMessages(1)
	if err := h.messageQueue.Enqueue(request.Context(), message); err != nil {
		h.metrics.IncWebhookEnqueueFailure()
		h.logger.Error("enqueue twilio webhook message failed", map[string]any{"phone_number": message.PhoneNumber, "message_id": message.MessageID, "error": err.Error()})
		writeError(responseWriter, http.StatusInternalServerError, "failed to enqueue webhook message")
		return
	}

	writeJSON(responseWriter, http.StatusOK, WebhookResponse{Status: "received"})
}

func (h *Handler) handleSimulation(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(responseWriter, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	h.metrics.IncSimulationRequests()

	var simulationRequest SimulationRequest
	if err := json.NewDecoder(request.Body).Decode(&simulationRequest); err != nil {
		h.metrics.IncSimulationFailures()
		writeError(responseWriter, http.StatusBadRequest, "invalid json")
		return
	}

	simulationRequest.PhoneNumber = chat.NormalizePhoneNumber(simulationRequest.PhoneNumber)
	simulationRequest.Message = strings.TrimSpace(simulationRequest.Message)
	if simulationRequest.PhoneNumber == "" || simulationRequest.Message == "" {
		h.metrics.IncSimulationFailures()
		writeError(responseWriter, http.StatusBadRequest, "fields 'phone_number' and 'message' are required")
		return
	}

	reply, err := h.chatbotService.BuildReply(request.Context(), simulationRequest.PhoneNumber, simulationRequest.Message)
	if err != nil {
		h.metrics.IncSimulationFailures()
		statusCode := http.StatusInternalServerError
		if errors.Is(err, chat.ErrPhoneNumberNotAllowed) {
			statusCode = http.StatusForbidden
		}
		h.logger.Error("simulation failed", map[string]any{"phone_number": simulationRequest.PhoneNumber, "error": err.Error()})
		writeError(responseWriter, statusCode, err.Error())
		return
	}

	writeJSON(responseWriter, http.StatusOK, SimulationResponse{PhoneNumber: simulationRequest.PhoneNumber, InputMessage: simulationRequest.Message, ReplyMessage: reply})
}

func writeJSON(responseWriter http.ResponseWriter, statusCode int, payload any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(payload)
}

func writeError(responseWriter http.ResponseWriter, statusCode int, message string) {
	writeJSON(responseWriter, statusCode, ErrorResponse{Error: message})
}

func (h *Handler) resolveTwilioWebhookURL(request *http.Request) string {
	if configured := strings.TrimSpace(h.config.TwilioWebhookBaseURL); configured != "" {
		if strings.HasSuffix(configured, request.URL.Path) {
			return configured
		}
		return configured + request.URL.Path
	}

	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = strings.Split(forwardedProto, ",")[0]
	}

	host := strings.TrimSpace(request.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = request.Host
	}

	return scheme + "://" + host + request.URL.RequestURI()
}
