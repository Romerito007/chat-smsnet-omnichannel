// Package entity holds the Sector aggregate.
package entity

import "time"

// Sector is a department/area within a tenant (e.g. Sales, Support). It groups
// queues and agents. Business hours live on the ChannelConnection, not here.
type Sector struct {
	ID          string
	TenantID    string
	Name        string
	Description string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
