package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	nooparchive "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/archive/noop"
	memoryidempotency "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/idempotency/memory"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/messaging/noop"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/reply/fallback"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/repository/memory"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

func TestHandleWebhookNotificationSignatureValidation(t *testing.T) {
	t.Parallel()

	payload := `{"entry":[{"changes":[{"value":{"contacts":[{"wa_id":"5511999999999"}],"messages":[{"id":"wamid.1","from":"5511999999999","type":"text","text":{"body":"hello"}}]}}]}]}`

	testCases := []struct {
		name            string
		appSecret       string
		signatureHeader string
		expectedStatus  int
	}{
		{name: "accepts payload when app secret is not configured", expectedStatus: http.StatusOK},
		{name: "accepts payload with a valid signature", appSecret: "top-secret", signatureHeader: buildSignatureHeader(payload, "top-secret"), expectedStatus: http.StatusOK},
		{name: "rejects payload without signature when app secret is configured", appSecret: "top-secret", expectedStatus: http.StatusUnauthorized},
		{name: "rejects payload with an invalid signature", appSecret: "top-secret", signatureHeader: "sha256=deadbeef", expectedStatus: http.StatusUnauthorized},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			handler := newTestHandler(config.Config{WhatsAppAppSecret: testCase.appSecret})
			request := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
			if testCase.signatureHeader != "" {
				request.Header.Set("X-Hub-Signature-256", testCase.signatureHeader)
			}
			recorder := httptest.NewRecorder()
			handler.handleWebhookNotification(recorder, request)
			if recorder.Code != testCase.expectedStatus {
				t.Fatalf("expected status %d, got %d, body=%s", testCase.expectedStatus, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestHandleMetricsReturnsSnapshot(t *testing.T) {
	handler := newTestHandler(config.Config{})
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()

	handler.handleMetrics(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}

func newTestHandler(cfg config.Config) *Handler {
	logger := observability.NewLogger()
	service := chatbot.NewService("", fallback.NewGenerator(), noop.NewSender(logger), memory.NewConversationRepository(12), nooparchive.NewMessageArchive(), memoryidempotency.NewStore(config.DefaultWebhookIdempotencyTTL, config.DefaultWebhookProcessingTTL))
	return NewHandler(service, cfg, logger, observability.NewMetrics())
}

func buildSignatureHeader(payload, appSecret string) string {
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write([]byte(payload))
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}
