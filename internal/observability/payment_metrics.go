package observability

import "sync/atomic"

// PaymentMetrics holds process-level counters for payment/webhook lifecycle monitoring.
type PaymentMetrics struct {
	webhookReceived   atomic.Uint64
	webhookRejected   atomic.Uint64
	webhookDuplicate  atomic.Uint64
	webhookProcessed  atomic.Uint64
	webhookFailed     atomic.Uint64
	paymentReconciled atomic.Uint64
}

// Snapshot is a read-only copy of metrics counters.
type Snapshot struct {
	WebhookReceived   uint64 `json:"webhook_received"`
	WebhookRejected   uint64 `json:"webhook_rejected"`
	WebhookDuplicate  uint64 `json:"webhook_duplicate"`
	WebhookProcessed  uint64 `json:"webhook_processed"`
	WebhookFailed     uint64 `json:"webhook_failed"`
	PaymentReconciled uint64 `json:"payment_reconciled"`
}

// PaymentLifecycleMetrics is a shared in-process metric collector.
var PaymentLifecycleMetrics = &PaymentMetrics{}

func (m *PaymentMetrics) IncWebhookReceived()  { m.webhookReceived.Add(1) }
func (m *PaymentMetrics) IncWebhookRejected()  { m.webhookRejected.Add(1) }
func (m *PaymentMetrics) IncWebhookDuplicate() { m.webhookDuplicate.Add(1) }
func (m *PaymentMetrics) IncWebhookProcessed() { m.webhookProcessed.Add(1) }
func (m *PaymentMetrics) IncWebhookFailed()    { m.webhookFailed.Add(1) }
func (m *PaymentMetrics) IncPaymentReconciled() {
	m.paymentReconciled.Add(1)
}

func (m *PaymentMetrics) Snapshot() Snapshot {
	return Snapshot{
		WebhookReceived:   m.webhookReceived.Load(),
		WebhookRejected:   m.webhookRejected.Load(),
		WebhookDuplicate:  m.webhookDuplicate.Load(),
		WebhookProcessed:  m.webhookProcessed.Load(),
		WebhookFailed:     m.webhookFailed.Load(),
		PaymentReconciled: m.paymentReconciled.Load(),
	}
}
