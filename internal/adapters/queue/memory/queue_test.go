package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

type stubProcessor struct {
	results chan processorResult
}

type processorResult struct {
	result chatbot.ProcessResult
	err    error
}

func (s *stubProcessor) ProcessIncomingMessage(_ context.Context, _ chat.IncomingMessage) (chatbot.ProcessResult, error) {
	result := <-s.results
	return result.result, result.err
}

func TestQueueRetriesTransientFailure(t *testing.T) {
	processor := &stubProcessor{results: make(chan processorResult, 2)}
	processor.results <- processorResult{err: errors.New("temporary failure")}
	processor.results <- processorResult{result: chatbot.ProcessResult{}}

	metrics := observability.NewMetrics()
	queue := NewQueue(1, 2, 1, 10*time.Millisecond, processor, observability.NewLogger(), metrics)

	if err := queue.Enqueue(context.Background(), chat.IncomingMessage{MessageID: "wamid.1", PhoneNumber: "5511999999999", Text: "hello"}); err != nil {
		t.Fatalf("expected nil enqueue error, got %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		snapshot := metrics.Snapshot()
		if snapshot.WebhookProcessed == 1 && snapshot.WebhookRetries == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	snapshot := metrics.Snapshot()
	t.Fatalf("expected processed=1 and retries=1, got processed=%d retries=%d failures=%d", snapshot.WebhookProcessed, snapshot.WebhookRetries, snapshot.WebhookFailures)
}

func TestQueuePropagatesCanceledContextOnEnqueue(t *testing.T) {
	processor := &stubProcessor{results: make(chan processorResult)}
	queue := NewQueue(1, 1, 0, 10*time.Millisecond, processor, observability.NewLogger(), observability.NewMetrics())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := queue.Enqueue(ctx, chat.IncomingMessage{MessageID: "wamid.2", PhoneNumber: "5511999999999", Text: "hello again"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
