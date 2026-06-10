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

// QueryLogRepository persists the minimal technical query log.
type QueryLogRepository interface {
	Create(ctx context.Context, l *entity.ProviderQueryLog) error
}
