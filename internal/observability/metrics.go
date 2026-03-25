package observability

import "sync/atomic"

type Metrics struct {
	webhookRequests       atomic.Uint64
	webhookMessages       atomic.Uint64
	webhookEnqueued       atomic.Uint64
	webhookEnqueueFailure atomic.Uint64
	webhookProcessed      atomic.Uint64
	webhookDuplicates     atomic.Uint64
	webhookFailures       atomic.Uint64
	webhookRetries        atomic.Uint64
	simulationRequests    atomic.Uint64
	simulationFailures    atomic.Uint64
}

type MetricsSnapshot struct {
	WebhookRequests       uint64 `json:"webhook_requests"`
	WebhookMessages       uint64 `json:"webhook_messages"`
	WebhookEnqueued       uint64 `json:"webhook_enqueued"`
	WebhookEnqueueFailure uint64 `json:"webhook_enqueue_failures"`
	WebhookProcessed      uint64 `json:"webhook_processed"`
	WebhookDuplicates     uint64 `json:"webhook_duplicates"`
	WebhookFailures       uint64 `json:"webhook_failures"`
	WebhookRetries        uint64 `json:"webhook_retries"`
	SimulationRequests    uint64 `json:"simulation_requests"`
	SimulationFailures    uint64 `json:"simulation_failures"`
}

func NewMetrics() *Metrics             { return &Metrics{} }
func (m *Metrics) IncWebhookRequests() { m.webhookRequests.Add(1) }
func (m *Metrics) AddWebhookMessages(total int) {
	if total > 0 {
		m.webhookMessages.Add(uint64(total))
	}
}
func (m *Metrics) IncWebhookEnqueued()       { m.webhookEnqueued.Add(1) }
func (m *Metrics) IncWebhookEnqueueFailure() { m.webhookEnqueueFailure.Add(1) }
func (m *Metrics) IncWebhookProcessed()      { m.webhookProcessed.Add(1) }
func (m *Metrics) IncWebhookDuplicates()     { m.webhookDuplicates.Add(1) }
func (m *Metrics) IncWebhookFailures()       { m.webhookFailures.Add(1) }
func (m *Metrics) IncWebhookRetries()        { m.webhookRetries.Add(1) }
func (m *Metrics) IncSimulationRequests()    { m.simulationRequests.Add(1) }
func (m *Metrics) IncSimulationFailures()    { m.simulationFailures.Add(1) }

func (m *Metrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		WebhookRequests:       m.webhookRequests.Load(),
		WebhookMessages:       m.webhookMessages.Load(),
		WebhookEnqueued:       m.webhookEnqueued.Load(),
		WebhookEnqueueFailure: m.webhookEnqueueFailure.Load(),
		WebhookProcessed:      m.webhookProcessed.Load(),
		WebhookDuplicates:     m.webhookDuplicates.Load(),
		WebhookFailures:       m.webhookFailures.Load(),
		WebhookRetries:        m.webhookRetries.Load(),
		SimulationRequests:    m.simulationRequests.Load(),
		SimulationFailures:    m.simulationFailures.Load(),
	}
}
