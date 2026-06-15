package shared

import "context"

// WebhookEmitter emits an internal business event for outbound webhook delivery.
// It is implemented by the webhooks Dispatcher and defaults to a no-op so any
// domain can emit events without taking a hard dependency on the webhooks
// domain. Emission is fire-and-forget: a webhook failure must never break the
// primary operation that produced the event.
type WebhookEmitter interface {
	// Emit fans an internal event (e.g. "conversation.created") out to the tenant's
	// matching webhook subscriptions. sectorID is the event's sector ("" when none),
	// honored against a subscription's sector scopes. payload must be JSON-serializable.
	Emit(ctx context.Context, tenantID, event, sectorID string, payload any)
	// EmitLazy is Emit with a DEFERRED payload: build is invoked at most once, and
	// ONLY after the dispatcher has confirmed at least one subscription matches the
	// event. Callers whose payload is expensive to assemble (e.g. it resolves the
	// contact and agent blocks) use this so a tenant with no webhook pays nothing on
	// the hot path. build must return a JSON-serializable value.
	EmitLazy(ctx context.Context, tenantID, event, sectorID string, build func() any)
}

// NoopWebhookEmitter discards events. Useful as a default and in tests.
type NoopWebhookEmitter struct{}

// Emit implements WebhookEmitter.
func (NoopWebhookEmitter) Emit(context.Context, string, string, string, any) {}

// EmitLazy implements WebhookEmitter; it never invokes build (no subscriptions).
func (NoopWebhookEmitter) EmitLazy(context.Context, string, string, string, func() any) {}

// ChannelWebhookManager keeps a channel connection's MANAGED webhook subscription
// in sync. It is implemented by the webhooks SubscriptionService and injected into
// the channels ConnectionService, so a channel with an outbound URL produces a
// normal webhook (full pipeline) owned by the channel — instead of a separate
// outbound rail. All calls are tenant-scoped from ctx and best-effort by the
// caller's contract.
type ChannelWebhookManager interface {
	// SyncChannelWebhook upserts the channel's managed subscription: when url is
	// non-empty it creates or updates the subscription (url + secret) owned by the
	// channel; when url is empty it removes any existing managed subscription.
	SyncChannelWebhook(ctx context.Context, channelID, url, secret string) error
	// RemoveChannelWebhook deletes the channel's managed subscription, if any.
	RemoveChannelWebhook(ctx context.Context, channelID string) error
}

// NoopChannelWebhookManager does nothing. Default when the channels service is not
// wired to the webhooks domain (e.g. in unit tests).
type NoopChannelWebhookManager struct{}

// SyncChannelWebhook implements ChannelWebhookManager.
func (NoopChannelWebhookManager) SyncChannelWebhook(context.Context, string, string, string) error {
	return nil
}

// RemoveChannelWebhook implements ChannelWebhookManager.
func (NoopChannelWebhookManager) RemoveChannelWebhook(context.Context, string) error { return nil }
