// Package entity holds the providerhub aggregates: the integration config and
// the minimal technical query log. No external provider payloads are persisted.
package entity

import "time"

// ProviderIntegrationConfig points at the tenant's standardized provider API.
// The secret (encrypted at rest) authenticates outbound queries.
type ProviderIntegrationConfig struct {
	ID        string
	TenantID  string
	Name      string
	BaseURL   string
	AuthType  string
	Secret    string
	Enabled   bool
	TimeoutMs int
	CreatedAt time.Time
	UpdatedAt time.Time
}
