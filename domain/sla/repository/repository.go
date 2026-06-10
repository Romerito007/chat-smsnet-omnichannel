// Package repository declares the SLA persistence contracts.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sla/entity"
)

// PolicyRepository persists SLA policies (tenant-scoped from the context).
type PolicyRepository interface {
	Create(ctx context.Context, p *entity.SLAPolicy) error
	Update(ctx context.Context, p *entity.SLAPolicy) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.SLAPolicy, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.SLAPolicy, error)
	// ListEnabled returns every enabled policy for the tenant (used for matching).
	ListEnabled(ctx context.Context) ([]*entity.SLAPolicy, error)
}

// TrackingRepository persists per-conversation SLA tracking.
type TrackingRepository interface {
	Create(ctx context.Context, t *entity.SLATracking) error
	Update(ctx context.Context, t *entity.SLATracking) error
	FindByConversation(ctx context.Context, conversationID string) (*entity.SLATracking, error)
	// ListAtRisk returns running trackings for the tenant whose unmet targets are
	// at/over the warning threshold or already breached.
	ListAtRisk(ctx context.Context, page shared.PageRequest) ([]*entity.SLATracking, error)
	// ListRunningAcrossTenants returns running trackings for every tenant. Used by
	// the sla.check scheduler job; it is intentionally NOT tenant-scoped.
	ListRunningAcrossTenants(ctx context.Context, limit int) ([]*entity.SLATracking, error)
}
