// Package repository declares the pipeline persistence contract.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
)

// PipelineRepository persists sales pipelines within a tenant (scope from ctx).
type PipelineRepository interface {
	Create(ctx context.Context, p *entity.Pipeline) error
	Update(ctx context.Context, p *entity.Pipeline) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.Pipeline, error)
	// List returns the tenant's pipelines (few per tenant; no pagination), default
	// first then by name.
	List(ctx context.Context) ([]*entity.Pipeline, error)
	// CountByTenant counts the tenant's pipelines (to make the first one default).
	CountByTenant(ctx context.Context) (int, error)
	// ClearDefault unsets is_default on every pipeline of the tenant except keepID
	// (pass "" to clear all), so only one default exists at a time.
	ClearDefault(ctx context.Context, keepID string) error
}
