// Package repository declares the automation persistence contracts.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// IntegrationRepository persists automation integrations.
type IntegrationRepository interface {
	Create(ctx context.Context, i *entity.AutomationIntegration) error
	Update(ctx context.Context, i *entity.AutomationIntegration) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.AutomationIntegration, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.AutomationIntegration, error)
	// FindEnabled returns the tenant's first enabled integration.
	FindEnabled(ctx context.Context) (*entity.AutomationIntegration, error)
}

// RunRepository persists automation runs.
type RunRepository interface {
	Create(ctx context.Context, r *entity.AutomationRun) error
	Update(ctx context.Context, r *entity.AutomationRun) error
	FindByID(ctx context.Context, id string) (*entity.AutomationRun, error)
	FindByExternalRunID(ctx context.Context, externalRunID string) (*entity.AutomationRun, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.AutomationRun, error)
}
