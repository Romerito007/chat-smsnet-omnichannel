// Package repository declares the deal persistence contract.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// DealRepository persists sales deals within a tenant (scope from ctx).
type DealRepository interface {
	Create(ctx context.Context, d *entity.Deal) error
	Update(ctx context.Context, d *entity.Deal) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.Deal, error)
	// List returns a keyset page of deals matching the filter, constrained by the
	// actor's visibility (assigned-to-me or my sectors when not all-scope).
	List(ctx context.Context, f contracts.ListFilter, vis contracts.Visibility, page shared.PageRequest) ([]*entity.Deal, error)
	// CountByStage counts deals in a pipeline stage (the StageDealChecker for the
	// pipelines domain: refuse deleting a non-empty stage).
	CountByStage(ctx context.Context, pipelineID, stageID string) (int, error)
}
