// Package repository declares the deal persistence contract.
package repository

import (
	"context"
	"time"

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

	// ── sales metrics (aggregations, visibility-scoped) ──────────────────────
	// OpenByStage groups OPEN deals by stage: count + value sum.
	OpenByStage(ctx context.Context, f contracts.SalesFilter, vis contracts.Visibility) ([]contracts.FunnelStage, error)
	// ClosedTotals returns the won/lost count+value closed within the period.
	ClosedTotals(ctx context.Context, status string, f contracts.SalesFilter, vis contracts.Visibility) (contracts.CountValue, error)
	// OpenByAgent groups OPEN deals by assigned_to: count + value sum.
	OpenByAgent(ctx context.Context, f contracts.SalesFilter, vis contracts.Visibility) (map[string]contracts.CountValue, error)
	// ClosedByAgent groups won/lost (closed in period) by assigned_to.
	ClosedByAgent(ctx context.Context, status string, f contracts.SalesFilter, vis contracts.Visibility) (map[string]contracts.CountValue, error)
	// AvgCloseSeconds is the mean (closed_at - created_at) over deals won in period.
	AvgCloseSeconds(ctx context.Context, f contracts.SalesFilter, vis contracts.Visibility) (avg float64, wonCount int, err error)
	// StageDwell is the mean current dwell time of OPEN deals per stage (now -
	// stage_changed_at).
	StageDwell(ctx context.Context, now time.Time, f contracts.SalesFilter, vis contracts.Visibility) ([]contracts.StageDwell, error)
	// StalledOpen returns open deals whose stage has not changed since `before`.
	StalledOpen(ctx context.Context, before time.Time, limit int, f contracts.SalesFilter, vis contracts.Visibility) ([]*entity.Deal, error)
}
