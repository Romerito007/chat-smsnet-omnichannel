// Package entity holds the Sector aggregate.
package entity

import "time"

// Sector is a department/area within a tenant (e.g. Sales, Support). It groups
// queues and agents and carries optional business hours (formalized later by the
// businesshours domain; stored here as a free-form document for now).
type Sector struct {
	ID            string
	TenantID      string
	Name          string
	Description   string
	Enabled       bool
	BusinessHours map[string]any
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
