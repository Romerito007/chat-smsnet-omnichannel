package shared

import "context"

// WebhookEmitter emits an internal business event for outbound webhook delivery.
// It is implemented by the webhooks Dispatcher and defaults to a no-op so any
// domain can emit events without taking a hard dependency on the webhooks
// domain. Emission is fire-and-forget: a webhook failure must never break the
// primary operation that produced the event.
type WebhookEmitter interface {
	// Emit fans an event (e.g. "conversation.created") out to the tenant's
	// matching webhook subscriptions. payload must be JSON-serializable.
	Emit(ctx context.Context, tenantID string, event string, payload any)
}

// NoopWebhookEmitter discards events. Useful as a default and in tests.
type NoopWebhookEmitter struct{}

// Emit implements WebhookEmitter.
func (NoopWebhookEmitter) Emit(context.Context, string, string, any) {}
