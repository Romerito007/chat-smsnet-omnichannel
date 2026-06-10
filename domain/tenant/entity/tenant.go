// Package entity holds the Tenant aggregate: the top-level isolation boundary.
package entity

import "time"

// Status is the tenant lifecycle state.
type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
)

// Tenant is a company/account. It is the root of multi-tenant isolation; every
// other entity references it via tenant_id.
type Tenant struct {
	ID        string
	Name      string
	Status    Status
	Settings  map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
}
