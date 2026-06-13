// Package repository declares the providerhub persistence contracts: the config
// and the minimal query log (no external payloads).
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ConfigRepository persists the provider integration config (one per tenant in
// practice; the service uses the enabled one).
type ConfigRepository interface {
	Create(ctx context.Context, c *entity.ProviderIntegrationConfig) error
	Update(ctx context.Context, c *entity.ProviderIntegrationConfig) error
	FindByID(ctx context.Context, id string) (*entity.ProviderIntegrationConfig, error)
	FindEnabled(ctx context.Context) (*entity.ProviderIntegrationConfig, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.ProviderIntegrationConfig, error)
}

// ProfileRepository persists the per-tenant ISP profiles (many per tenant). The
// ISP credentials are encrypted at rest.
type ProfileRepository interface {
	Create(ctx context.Context, p *entity.ISPProfile) error
	Update(ctx context.Context, p *entity.ISPProfile) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.ISPProfile, error)
	// FindDefault returns the tenant's default profile, or a NotFound apperror.
	FindDefault(ctx context.Context) (*entity.ISPProfile, error)
	// List returns all profiles for the tenant (no pagination: a tenant holds few).
	List(ctx context.Context) ([]*entity.ISPProfile, error)
	// ClearDefault unsets is_default on every profile of the tenant. Used before
	// marking a new default so the partial-unique index never sees two defaults.
	ClearDefault(ctx context.Context) error
}

// QueryLogRepository persists the minimal technical query log.
type QueryLogRepository interface {
	Create(ctx context.Context, l *entity.ProviderQueryLog) error
}
