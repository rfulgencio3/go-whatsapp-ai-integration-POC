package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	nooparchive "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/archive/noop"
	memoryidempotency "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/idempotency/memory"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/messaging/noop"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/reply/fallback"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/adapters/repository/memory"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type stubQueue struct {
	enqueueCount int
	err          error
}

func (s *stubQueue) Enqueue(_ context.Context, _ chat.IncomingMessage) error {
	s.enqueueCount++
	return s.err
}

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
			handler, _ := newTestHandler(config.Config{WhatsAppAppSecret: testCase.appSecret})
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

func TestHandleWebhookNotificationEnqueuesMessages(t *testing.T) {
	handler, queue := newTestHandler(config.Config{})
	payload := `{"entry":[{"changes":[{"value":{"contacts":[{"wa_id":"5511999999999"}],"messages":[{"id":"wamid.1","from":"5511999999999","type":"text","text":{"body":"hello"}}]}}]}]}`
	request := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	recorder := httptest.NewRecorder()

	handler.handleWebhookNotification(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if queue.enqueueCount != 1 {
		t.Fatalf("expected 1 enqueued message, got %d", queue.enqueueCount)
	}
}

func TestHandleTwilioWebhookNotificationEnqueuesMessage(t *testing.T) {
	cfg := config.Config{TwilioAuthToken: "twilio-secret"}
	handler, queue := newTestHandler(cfg)
	payload := "MessageSid=SM123&WaId=5511999999999&Body=hello&NumMedia=0"
	request := httptest.NewRequest(http.MethodPost, "/webhook/twilio", strings.NewReader(payload))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-Twilio-Signature", buildTwilioSignature("http://example.com/webhook/twilio", payload, cfg.TwilioAuthToken))
	recorder := httptest.NewRecorder()

	handler.handleTwilioWebhookNotification(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if queue.enqueueCount != 1 {
		t.Fatalf("expected 1 enqueued message, got %d", queue.enqueueCount)
	}
}

func TestHandleTwilioWebhookNotificationRejectsInvalidSignature(t *testing.T) {
	cfg := config.Config{TwilioAuthToken: "twilio-secret"}
	handler, _ := newTestHandler(cfg)
	payload := "MessageSid=SM123&WaId=5511999999999&Body=hello&NumMedia=0"
	request := httptest.NewRequest(http.MethodPost, "/webhook/twilio", strings.NewReader(payload))
	request.Header.Set("X-Twilio-Signature", "invalid")
	recorder := httptest.NewRecorder()

	handler.handleTwilioWebhookNotification(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestHandlePrivacyPolicyReturnsHTML(t *testing.T) {
	handler, _ := newTestHandler(config.Config{})
	request := httptest.NewRequest(http.MethodGet, "/privacy-policy", nil)
	recorder := httptest.NewRecorder()

	handler.handlePrivacyPolicy(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected html content type, got %q", contentType)
	}
	if !strings.Contains(recorder.Body.String(), "Privacy Policy") {
		t.Fatalf("expected privacy policy content, got %s", recorder.Body.String())
	}
}

func TestHandleMetricsReturnsSnapshot(t *testing.T) {
	handler, _ := newTestHandler(config.Config{})
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()

	handler.handleMetrics(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}

func newTestHandler(cfg config.Config) (*Handler, *stubQueue) {
	logger := observability.NewLogger()
	service := chatbot.NewService("", fallback.NewGenerator(), nil, noop.NewSender(logger), nil, memory.NewConversationRepository(12), nooparchive.NewMessageArchive(), memoryidempotency.NewStore(config.DefaultWebhookIdempotencyTTL, config.DefaultWebhookProcessingTTL))
	queue := &stubQueue{}
	return NewHandler(service, queue, cfg, logger, observability.NewMetrics()), queue
}

func buildSignatureHeader(payload, appSecret string) string {
	mac := hmac.New(sha256.New, []byte(appSecret))
	_, _ = mac.Write([]byte(payload))
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

func buildTwilioSignature(requestURL, payload, authToken string) string {
	values, err := url.ParseQuery(payload)
	if err != nil {
		return ""
	}
	return computeTwilioSignature(requestURL, values, authToken)
}
