// Package entity holds the monitoring aggregates: the integration config and the
// minimal technical query log. No external monitoring payloads are persisted.
package entity

import "time"

// MonitoringIntegrationConfig points at the tenant's external monitoring system.
// The secret (encrypted at rest) authenticates outbound queries.
type MonitoringIntegrationConfig struct {
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
