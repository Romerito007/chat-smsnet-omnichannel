// Package entity holds the privacy (LGPD) aggregates: the per-tenant
// RetentionPolicy, data-export requests and legal holds.
package entity

import "time"

// RetentionPolicy is the per-tenant data-retention configuration applied by the
// privacy.retention scheduler job. Each value is a number of days; 0 means
// "keep indefinitely" (the category is skipped). Data under an active legal hold
// is never deleted before its deadline.
type RetentionPolicy struct {
	TenantID string
	// MessagesDays caps how long messages are kept.
	MessagesDays int
	// ClosedConversationsDays caps how long closed/resolved/archived
	// conversations are kept (by their closed_at).
	ClosedConversationsDays int
	// TechnicalLogsDays caps how long conversation timeline events (technical
	// logs) are kept.
	TechnicalLogsDays int
	// AuditLogsDays caps how long audit logs are kept.
	AuditLogsDays int
	// NotificationsDays caps how long in-app notifications are kept.
	NotificationsDays int
	UpdatedAt         time.Time
}

// DefaultRetention returns an all-zero (keep-forever) policy for a tenant that
// has not configured one yet.
func DefaultRetention(tenantID string) *RetentionPolicy {
	return &RetentionPolicy{TenantID: tenantID}
}

// Normalize clamps negative values to zero.
func (p *RetentionPolicy) Normalize() {
	for _, v := range []*int{
		&p.MessagesDays, &p.ClosedConversationsDays, &p.TechnicalLogsDays,
		&p.AuditLogsDays, &p.NotificationsDays,
	} {
		if *v < 0 {
			*v = 0
		}
	}
}
