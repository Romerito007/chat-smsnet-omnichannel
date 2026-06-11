package shared

import (
	"context"
	"time"
)

// Actor types recorded on audit entries.
const (
	ActorTypeUser   = "user"   // an authenticated operator
	ActorTypeSystem = "system" // an internal job / system actor
	ActorTypePublic = "public" // unauthenticated / public token
)

// AuditEntry is an immutable record of a security/compliance-relevant action.
// Producers (auth, iam, webhooks, conversations, providerhub, copilot, privacy…)
// build it; the audit domain persists it to the tenant-scoped audit_logs
// collection. ActorID/ActorType, IP and UserAgent are filled from the request
// context by the recorder when the producer leaves them empty.
type AuditEntry struct {
	TenantID     string
	ActorID      string         // user id, or "system" for internal jobs
	ActorType    string         // user | system | public
	Action       string         // e.g. "privacy.contact.anonymized"
	ResourceType string         // e.g. "contact", "user", "webhook"
	ResourceID   string         // affected resource id (optional)
	IP           string         // client ip (filled from context when empty)
	UserAgent    string         // client user-agent (filled from context when empty)
	Data         map[string]any // additional, non-sensitive context
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

// auditCtxKey is unexported to avoid collisions.
type auditCtxKey int

const auditMetaKey auditCtxKey = iota

// auditMeta carries request-scoped audit metadata captured at the HTTP border.
type auditMeta struct {
	IP        string
	UserAgent string
}

// WithAuditMeta stores the client ip and user-agent on the context so the audit
// recorder can attach them to entries produced deep in the service layer.
func WithAuditMeta(ctx context.Context, ip, userAgent string) context.Context {
	return context.WithValue(ctx, auditMetaKey, auditMeta{IP: ip, UserAgent: userAgent})
}

// AuditMetaFrom extracts the client ip and user-agent captured at the border.
func AuditMetaFrom(ctx context.Context) (ip, userAgent string) {
	if m, ok := ctx.Value(auditMetaKey).(auditMeta); ok {
		return m.IP, m.UserAgent
	}
	return "", ""
}
