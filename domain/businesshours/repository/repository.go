// Package repository declares the businesshours persistence contract.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// HolidayRepository persists holidays (tenant-scoped from the context).
type HolidayRepository interface {
	Create(ctx context.Context, h *entity.Holiday) error
	Update(ctx context.Context, h *entity.Holiday) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.Holiday, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.Holiday, error)
	// ListAll returns every holiday for the tenant (used by the status check,
	// where the holiday set per tenant is small).
	ListAll(ctx context.Context) ([]*entity.Holiday, error)
}
