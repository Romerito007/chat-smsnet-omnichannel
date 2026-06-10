// Package entity holds the automation domain aggregates: AutomationIntegration
// (the external flow connection) and AutomationRun (one execution).
package entity

import "time"

// AutomationIntegration is the configuration to reach the tenant's external flow
// system. The secret (encrypted at rest) signs/verifies traffic.
type AutomationIntegration struct {
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
