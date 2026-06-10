package contracts

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
)

// Gateway is the on-demand client to the tenant's standardized provider API.
// The implementation lives in infra/providerhub and talks ONLY to that API —
// never to IXC/SGP/MK/Voalle directly. It performs no caching or persistence.
type Gateway interface {
	GetCustomerProfile(ctx context.Context, cfg *entity.ProviderIntegrationConfig, lookup Lookup) (CustomerProfile, error)
	GetContracts(ctx context.Context, cfg *entity.ProviderIntegrationConfig, lookup Lookup) ([]Contract, error)
	GetFinancialStatus(ctx context.Context, cfg *entity.ProviderIntegrationConfig, lookup Lookup) (FinancialStatus, error)
	GetConnectionStatus(ctx context.Context, cfg *entity.ProviderIntegrationConfig, lookup Lookup) (ConnectionStatus, error)
	GetTickets(ctx context.Context, cfg *entity.ProviderIntegrationConfig, lookup Lookup) ([]Ticket, error)
	OpenTicket(ctx context.Context, cfg *entity.ProviderIntegrationConfig, lookup Lookup, input OpenTicketInput) (Ticket, error)
	// Ping verifies connectivity for the config test.
	Ping(ctx context.Context, cfg *entity.ProviderIntegrationConfig) error
}

// RateLimiter caps the per-tenant rate of outbound provider queries, protecting
// the upstream API. The implementation lives in infra/redis.
type RateLimiter interface {
	// Allow reports whether another query is permitted for the tenant right now.
	Allow(ctx context.Context, tenantID string) (bool, error)
}

// CreateConfig registers a provider integration config.
type CreateConfig struct {
	Name      string
	BaseURL   string
	AuthType  string
	Secret    string
	TimeoutMs int
}

// UpdateConfig carries optional fields; nil pointers mean "leave unchanged".
type UpdateConfig struct {
	Name      *string
	BaseURL   *string
	AuthType  *string
	Secret    *string
	Enabled   *bool
	TimeoutMs *int
}
