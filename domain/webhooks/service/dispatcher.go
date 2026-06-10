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
// asynchronous, signed delivery. Unknown events and tenants with no matching
// subscription are no-ops.
func (d *Dispatcher) Emit(ctx context.Context, tenantID string, event string, payload any) {
	if tenantID == "" || !entity.IsSupportedEvent(event) {
		return
	}
	subs, err := d.subs.ListEnabledByEvent(ctx, tenantID, event)
	if err != nil || len(subs) == 0 {
		return
	}

	now := d.clock.Now()
	for _, sub := range subs {
		id := shared.NewID()
		body, berr := buildEnvelope(id, event, now, payload)
		if berr != nil {
			continue
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
			continue
		}
		if d.enqueuer != nil {
			_ = d.enqueuer.EnqueueDeliver(contracts.DeliverTask{TenantID: tenantID, DeliveryID: delivery.ID})
		}
	}
}

var _ shared.WebhookEmitter = (*Dispatcher)(nil)
