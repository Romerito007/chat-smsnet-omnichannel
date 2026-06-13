// Package service holds the webhooks business logic: subscription CRUD, the
// internal-event dispatcher, and the delivery worker with retry/backoff and
// dead-lettering.
package service

import (
	"context"

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
	if tenantID == "" {
		return
	}
	wire, ok := entity.WireEvent(event)
	if !ok {
		return // not a webhook event
	}
	subs, err := d.subs.ListEnabledByEvent(ctx, tenantID, wire)
	if err != nil || len(subs) == 0 {
		return
	}

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
