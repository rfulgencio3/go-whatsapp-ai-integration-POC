package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type Handler struct {
	chatbotService *chatbot.Service
	config         config.Config
	logger         *observability.Logger
	metrics        *observability.Metrics
}

func NewHandler(chatbotService *chatbot.Service, cfg config.Config, logger *observability.Logger, metrics *observability.Metrics) *Handler {
	if metrics == nil {
		metrics = observability.NewMetrics()
	}

	return &Handler{chatbotService: chatbotService, config: cfg, logger: logger, metrics: metrics}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", h.handleRoot)
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/metrics", h.handleMetrics)
	mux.HandleFunc("/webhook", h.handleWebhook)
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
	writeJSON(responseWriter, http.StatusOK, HealthResponse{Status: "ok", WhatsAppSenderConfigured: h.config.HasWhatsAppSenderConfig(), WhatsAppWebhookConfigured: h.config.HasWhatsAppWebhookConfig(), GeminiConfigured: h.config.HasGeminiConfig(), GeminiModel: h.config.GeminiModel})
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
		result, err := h.chatbotService.ProcessIncomingMessage(request.Context(), message)
		if err != nil {
			h.metrics.IncWebhookFailures()
			h.logger.Error("process incoming message failed", map[string]any{"phone_number": message.PhoneNumber, "message_id": message.MessageID, "error": err.Error()})
			continue
		}
		if result.Duplicate {
			h.metrics.IncWebhookDuplicates()
			h.logger.Info("duplicate webhook message skipped", map[string]any{"phone_number": message.PhoneNumber, "message_id": message.MessageID})
			continue
		}
		h.metrics.IncWebhookProcessed()
		h.logger.Info("webhook message processed", map[string]any{"phone_number": message.PhoneNumber, "message_id": message.MessageID})
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
