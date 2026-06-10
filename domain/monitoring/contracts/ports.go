package contracts

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/entity"
)

// Gateway is the on-demand client to the tenant's external monitoring system. It
// performs no caching or persistence and returns normalized DTOs only.
type Gateway interface {
	GetSummary(ctx context.Context, cfg *entity.MonitoringIntegrationConfig, lookup Lookup) (MonitoringSummary, error)
	GetIncidents(ctx context.Context, cfg *entity.MonitoringIntegrationConfig, lookup Lookup) ([]Incident, error)
	// Ping verifies connectivity for the config test.
	Ping(ctx context.Context, cfg *entity.MonitoringIntegrationConfig) error
}

// RateLimiter caps the per-tenant rate of outbound monitoring queries.
type RateLimiter interface {
	Allow(ctx context.Context, tenantID string) (bool, error)
}

// SaveConfig is the input to create-or-update the monitoring config (PATCH
// behaves as an upsert). Nil pointers leave existing values unchanged.
type SaveConfig struct {
	Name      *string
	BaseURL   *string
	AuthType  *string
	Secret    *string
	Enabled   *bool
	TimeoutMs *int
}
