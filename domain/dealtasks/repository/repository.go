// Package repository declares the deal-task persistence contract.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// TaskRepository persists deal tasks within a tenant (scope from ctx).
type TaskRepository interface {
	Create(ctx context.Context, t *entity.DealTask) error
	Update(ctx context.Context, t *entity.DealTask) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.DealTask, error)
	// ListByDeal returns a keyset page of a deal's tasks (most recent first).
	ListByDeal(ctx context.Context, dealID string, page shared.PageRequest) ([]*entity.DealTask, error)
	// List returns a keyset page of the tenant's tasks matching the filter — the
	// consolidated "my tasks" view (most recent first).
	List(ctx context.Context, f contracts.ListFilter, page shared.PageRequest) ([]*entity.DealTask, error)
}
