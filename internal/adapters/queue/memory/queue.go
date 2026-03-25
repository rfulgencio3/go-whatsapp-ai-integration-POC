package memory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/config"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/chat"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/observability"
	"github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/usecase/chatbot"
)

var errQueueFull = errors.New("webhook queue is full")

type queuedMessage struct {
	message chat.IncomingMessage
	attempt int
}

type Queue struct {
	processor  chatbot.MessageProcessor
	logger     *observability.Logger
	metrics    *observability.Metrics
	jobs       chan queuedMessage
	maxRetries int
	retryDelay time.Duration
	context    context.Context
}

func NewQueue(workerCount, bufferSize, maxRetries int, retryDelay time.Duration, processor chatbot.MessageProcessor, logger *observability.Logger, metrics *observability.Metrics) *Queue {
	if workerCount <= 0 {
		workerCount = config.DefaultWebhookQueueWorkers
	}
	if bufferSize <= 0 {
		bufferSize = config.DefaultWebhookQueueBufferSize
	}
	if maxRetries < 0 {
		maxRetries = config.DefaultWebhookQueueMaxRetries
	}
	if retryDelay <= 0 {
		retryDelay = config.DefaultWebhookQueueRetryDelay
	}
	if metrics == nil {
		metrics = observability.NewMetrics()
	}

	queue := &Queue{processor: processor, logger: logger, metrics: metrics, jobs: make(chan queuedMessage, bufferSize), maxRetries: maxRetries, retryDelay: retryDelay, context: context.Background()}
	for workerIndex := 0; workerIndex < workerCount; workerIndex++ {
		go queue.runWorker(workerIndex + 1)
	}
	return queue
}

func (q *Queue) Enqueue(ctx context.Context, message chat.IncomingMessage) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("enqueue webhook message: %w", err)
	}

	job := queuedMessage{message: message, attempt: 1}
	select {
	case q.jobs <- job:
		q.metrics.IncWebhookEnqueued()
		return nil
	default:
		q.metrics.IncWebhookEnqueueFailure()
		return errQueueFull
	}
}

func (q *Queue) runWorker(workerID int) {
	for job := range q.jobs {
		q.handleJob(workerID, job)
	}
}

func (q *Queue) handleJob(workerID int, job queuedMessage) {
	result, err := q.processor.ProcessIncomingMessage(q.context, job.message)
	if err != nil {
		q.handleFailure(workerID, job, err)
		return
	}
	if result.Duplicate {
		q.metrics.IncWebhookDuplicates()
		q.logger.Info("duplicate webhook message skipped", map[string]any{"worker_id": workerID, "phone_number": job.message.PhoneNumber, "message_id": job.message.MessageID})
		return
	}
	q.metrics.IncWebhookProcessed()
	q.logger.Info("webhook message processed", map[string]any{"worker_id": workerID, "phone_number": job.message.PhoneNumber, "message_id": job.message.MessageID, "attempt": job.attempt})
}

func (q *Queue) handleFailure(workerID int, job queuedMessage, err error) {
	if job.attempt <= q.maxRetries {
		delay := q.retryDelay * time.Duration(job.attempt)
		q.metrics.IncWebhookRetries()
		q.logger.Error("webhook job failed, scheduling retry", map[string]any{"worker_id": workerID, "phone_number": job.message.PhoneNumber, "message_id": job.message.MessageID, "attempt": job.attempt, "max_retries": q.maxRetries, "retry_in": delay.String(), "original_error": err.Error()})
		q.scheduleRetry(queuedMessage{message: job.message, attempt: job.attempt + 1}, delay)
		return
	}
	q.metrics.IncWebhookFailures()
	q.logger.Error("webhook job failed permanently", map[string]any{"worker_id": workerID, "phone_number": job.message.PhoneNumber, "message_id": job.message.MessageID, "attempt": job.attempt, "error": err.Error()})
}

func (q *Queue) scheduleRetry(job queuedMessage, delay time.Duration) {
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
		q.jobs <- job
	}()
}
