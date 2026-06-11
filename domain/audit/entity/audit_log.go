// Package entity holds the AuditLog aggregate: an immutable record of a
// compliance-relevant action within a tenant.
package entity

import "time"

// AuditLog is one persisted audit entry. Records are append-only; retention is
// enforced by the audit.compact job and the privacy RetentionPolicy.
type AuditLog struct {
	ID           string
	TenantID     string
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	Metadata     map[string]any
	CreatedAt    time.Time
}
