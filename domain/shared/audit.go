package shared

import (
	"context"
	"time"
)

// AuditEntry is an immutable record of a security/compliance-relevant action.
// Producers (privacy, iam, channels…) build it; the audit domain persists it to
// the tenant-scoped audit_logs collection. The tenant and actor are taken from
// the entry (the producer reads them from context) so the port stays simple.
type AuditEntry struct {
	TenantID     string
	ActorID      string         // user id, or "system" for internal jobs
	Action       string         // e.g. "privacy.contact.anonymized"
	ResourceType string         // e.g. "contact", "retention_policy"
	ResourceID   string         // affected resource id (optional)
	Metadata     map[string]any // additional, non-sensitive context
	At           time.Time      // optional; the recorder fills it when zero
}

// Auditor persists audit entries. It is implemented by the audit domain and
// consulted by any domain that must leave a trail ("toda ação auditada"). Unlike
// the fire-and-forget Notifier, Record returns an error so callers performing a
// compliance action (LGPD export/anonymization) can treat a failed audit as a
// failed operation. The default no-op drops entries (tests, optional wiring).
type Auditor interface {
	Record(ctx context.Context, e AuditEntry) error
}

// NoopAuditor discards audit entries.
type NoopAuditor struct{}

// Record implements Auditor.
func (NoopAuditor) Record(context.Context, AuditEntry) error { return nil }
