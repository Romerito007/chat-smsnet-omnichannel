// Package repository declares the sector persistence contract.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// SectorRepository persists sectors within a tenant (scope from context).
type SectorRepository interface {
	Create(ctx context.Context, s *entity.Sector) error
	Update(ctx context.Context, s *entity.Sector) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.Sector, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.Sector, error)
}
