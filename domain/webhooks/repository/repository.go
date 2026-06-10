// Package repository declares the webhooks persistence contracts.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
)

// SubscriptionRepository persists webhook subscriptions (secret encrypted at
// rest). All reads are tenant-scoped from the context.
type SubscriptionRepository interface {
	Create(ctx context.Context, s *entity.WebhookSubscription) error
	Update(ctx context.Context, s *entity.WebhookSubscription) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.WebhookSubscription, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.WebhookSubscription, error)
	// ListEnabledByEvent returns every enabled subscription for the tenant that
	// listens for the given event. Used by the dispatcher on event emission.
	ListEnabledByEvent(ctx context.Context, tenantID, event string) ([]*entity.WebhookSubscription, error)
}

// DeliveryRepository persists per-attempt delivery records.
type DeliveryRepository interface {
	Create(ctx context.Context, d *entity.WebhookDelivery) error
	Update(ctx context.Context, d *entity.WebhookDelivery) error
	FindByID(ctx context.Context, id string) (*entity.WebhookDelivery, error)
	ListByWebhook(ctx context.Context, webhookID string, page shared.PageRequest) ([]*entity.WebhookDelivery, error)
}
