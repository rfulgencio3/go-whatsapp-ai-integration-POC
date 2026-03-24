package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type Handler struct {
	chatbotService *chatbot.Service
	config         config.Config
	logger         *log.Logger
}

func NewHandler(chatbotService *chatbot.Service, cfg config.Config, logger *log.Logger) *Handler {
	return &Handler{
		chatbotService: chatbotService,
		config:         cfg,
		logger:         logger,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", h.handleRoot)
	mux.HandleFunc("/healthz", h.handleHealth)
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
	writeJSON(responseWriter, http.StatusOK, HealthResponse{
		Status:                    "ok",
		WhatsAppSenderConfigured:  h.config.HasWhatsAppSenderConfig(),
		WhatsAppWebhookConfigured: h.config.HasWhatsAppWebhookConfig(),
		GeminiConfigured:          h.config.HasGeminiConfig(),
		GeminiModel:               h.config.GeminiModel,
	})
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
	var notification WhatsAppWebhookNotification
	if err := json.NewDecoder(request.Body).Decode(&notification); err != nil {
		writeError(responseWriter, http.StatusBadRequest, "invalid json")
		return
	}

	for _, message := range notification.ExtractIncomingMessages() {
		if err := h.chatbotService.ProcessIncomingMessage(request.Context(), message); err != nil {
			h.logger.Printf("process incoming message failed: phone_number=%s err=%v", message.PhoneNumber, err)
		}
	}

	writeJSON(responseWriter, http.StatusOK, WebhookResponse{Status: "received"})
}

func (h *Handler) handleSimulation(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(responseWriter, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var simulationRequest SimulationRequest
	if err := json.NewDecoder(request.Body).Decode(&simulationRequest); err != nil {
		writeError(responseWriter, http.StatusBadRequest, "invalid json")
		return
	}

	simulationRequest.PhoneNumber = chat.NormalizePhoneNumber(simulationRequest.PhoneNumber)
	simulationRequest.Message = strings.TrimSpace(simulationRequest.Message)
	if simulationRequest.PhoneNumber == "" || simulationRequest.Message == "" {
		writeError(responseWriter, http.StatusBadRequest, "fields 'phone_number' and 'message' are required")
		return
	}

	reply, err := h.chatbotService.BuildReply(request.Context(), simulationRequest.PhoneNumber, simulationRequest.Message)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, chat.ErrPhoneNumberNotAllowed) {
			statusCode = http.StatusForbidden
		}

		writeError(responseWriter, statusCode, err.Error())
		return
	}

	writeJSON(responseWriter, http.StatusOK, SimulationResponse{
		PhoneNumber:  simulationRequest.PhoneNumber,
		InputMessage: simulationRequest.Message,
		ReplyMessage: reply,
	})
}

func writeJSON(responseWriter http.ResponseWriter, statusCode int, payload any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(payload)
}

func writeError(responseWriter http.ResponseWriter, statusCode int, message string) {
	writeJSON(responseWriter, statusCode, ErrorResponse{Error: message})
}
