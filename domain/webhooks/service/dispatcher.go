// Package service holds the webhooks business logic: subscription CRUD, the
// internal-event dispatcher, and the delivery worker with retry/backoff and
// dead-lettering.
package service

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/repository"
)

// Dispatcher fans internal business events out to the tenant's matching webhook
// subscriptions. It implements shared.WebhookEmitter so any domain can emit
// without depending on the webhooks domain. Emission is best-effort: failures
// are swallowed so the primary operation is never affected.
type Dispatcher struct {
	subs       repository.SubscriptionRepository
	deliveries repository.DeliveryRepository
	enqueuer   contracts.Enqueuer
	clock      shared.Clock
}

// NewDispatcher builds the dispatcher.
func NewDispatcher(
	subs repository.SubscriptionRepository,
	deliveries repository.DeliveryRepository,
	enqueuer contracts.Enqueuer,
	clock shared.Clock,
) *Dispatcher {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Dispatcher{subs: subs, deliveries: deliveries, enqueuer: enqueuer, clock: clock}
}

// Emit creates a delivery record per matching subscription and enqueues each for
// asynchronous, signed delivery. The internal event is mapped to its wire name;
// delivery is filtered by the subscription's events[] (only what it subscribed to)
// AND its sector scopes. Unknown events and no-match tenants are no-ops.
func (d *Dispatcher) Emit(ctx context.Context, tenantID, event, sectorID string, payload any) {
	d.emit(ctx, tenantID, event, sectorID, func() any { return payload })
}

// EmitLazy is Emit with a deferred payload builder. The builder runs at most once,
// and ONLY after ListEnabledByEvent confirms at least one matching subscription —
// so when no webhook is subscribed to the event, the builder (and any contact/agent
// resolution it performs) never runs. This is the gate that keeps the high-volume
// inbound message_created path free of lookups for tenants without webhooks.
func (d *Dispatcher) EmitLazy(ctx context.Context, tenantID, event, sectorID string, build func() any) {
	d.emit(ctx, tenantID, event, sectorID, build)
}

// emit is the shared core: it resolves the wire event, looks up the matching
// subscriptions and — only if there is at least one — invokes build ONCE to obtain
// the payload, then creates and enqueues a delivery per in-scope subscription.
func (d *Dispatcher) emit(ctx context.Context, tenantID, event, sectorID string, build func() any) {
	if tenantID == "" {
		return
	}
	wire, ok := entity.WireEvent(event)
	if !ok {
		return // not a webhook event
	}
	subs, err := d.subs.ListEnabledByEvent(ctx, tenantID, wire)
	if err != nil || len(subs) == 0 {
		return // no subscriber → build() is never called (zero contact/agent lookups)
	}

	payload := build()
	now := d.clock.Now()
	for _, sub := range subs {
		if !scopeAllows(sub.Scopes, sectorID) {
			continue // out of the subscription's sector scope
		}
		id := shared.NewID()
		body, berr := buildEnvelope(id, wire, now, payload)
		if berr != nil {
			continue
		}
		delivery := &entity.WebhookDelivery{
			ID:        id,
			TenantID:  tenantID,
			WebhookID: sub.ID,
			Event:     wire,
			Payload:   body,
			Status:    entity.DeliveryPending,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := d.deliveries.Create(ctx, delivery); err != nil {
			continue
		}
		if d.enqueuer != nil {
			_ = d.enqueuer.EnqueueDeliver(contracts.DeliverTask{TenantID: tenantID, DeliveryID: delivery.ID})
		}
	}
}

// EmitTo delivers an event to ONE specific webhook by id, bypassing the
// subscription's events[] filter — the caller (e.g. an automation rule) decides
// what to send where. It reuses the full delivery pipeline (HMAC signing,
// retry/backoff, dead-letter, per-webhook rate limit). The event string is used
// as-is as the wire name. Returns an error so the caller can log the outcome; the
// tenant is taken from ctx for the repository lookup.
func (d *Dispatcher) EmitTo(ctx context.Context, tenantID, webhookID, event string, payload any) error {
	sub, err := d.subs.FindByID(ctx, webhookID)
	if err != nil {
		return err
	}
	if !sub.Enabled {
		return apperror.Validation("webhook is disabled")
	}
	now := d.clock.Now()
	id := shared.NewID()
	body, err := buildEnvelope(id, event, now, payload)
	if err != nil {
		return err
	}
	delivery := &entity.WebhookDelivery{
		ID:        id,
		TenantID:  tenantID,
		WebhookID: sub.ID,
		Event:     event,
		Payload:   body,
		Status:    entity.DeliveryPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := d.deliveries.Create(ctx, delivery); err != nil {
		return err
	}
	if d.enqueuer != nil {
		_ = d.enqueuer.EnqueueDeliver(contracts.DeliverTask{TenantID: tenantID, DeliveryID: delivery.ID})
	}
	return nil
}

// scopeAllows applies a subscription's sector scopes: empty scopes = every sector;
// otherwise the event's sector must be listed. Events with no sector (e.g.
// automation) are delivered only to unscoped subscriptions.
func scopeAllows(scopes []string, sectorID string) bool {
	if len(scopes) == 0 {
		return true
	}
	if sectorID == "" {
		return false
	}
	for _, s := range scopes {
		if s == sectorID {
			return true
		}
	}
	return false
}

var _ shared.WebhookEmitter = (*Dispatcher)(nil)
