// Package service implements the privacy (LGPD) domain: contact data export,
// anonymization and per-tenant retention. Every action is audited
// ("toda ação auditada") and anonymization refuses contacts under an active
// legal hold ("não anonimizar dados sob obrigação legal antes do prazo").
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// defaultURLTTL bounds how long an export's signed URL stays valid.
const defaultURLTTL = 24 * time.Hour

// Service orchestrates the privacy use cases.
type Service struct {
	store    repository.Store
	files    contracts.FileStore
	enqueuer contracts.ExportEnqueuer
	auditor  shared.Auditor
	clock    shared.Clock
	urlTTL   time.Duration
}

// NewService builds the service. A nil auditor/clock falls back to safe
// defaults; urlTTL <= 0 uses the 24h default.
func NewService(store repository.Store, files contracts.FileStore, enqueuer contracts.ExportEnqueuer, auditor shared.Auditor, clock shared.Clock, urlTTL time.Duration) *Service {
	if auditor == nil {
		auditor = shared.NoopAuditor{}
	}
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if urlTTL <= 0 {
		urlTTL = defaultURLTTL
	}
	return &Service{store: store, files: files, enqueuer: enqueuer, auditor: auditor, clock: clock, urlTTL: urlTTL}
}

// RequestExport records a pending export request and enqueues the privacy.export
// job. The actual file is assembled asynchronously by RunExport.
func (s *Service) RequestExport(ctx context.Context, contactID string) (*entity.ExportRequest, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	contactID = strings.TrimSpace(contactID)
	if contactID == "" {
		return nil, apperror.Validation("contact id is required")
	}
	req := &entity.ExportRequest{
		ID:          shared.NewID(),
		TenantID:    tenantID,
		ContactID:   contactID,
		Status:      entity.ExportPending,
		RequestedBy: actorID(ctx),
		CreatedAt:   s.clock.Now(),
	}
	if err := s.store.CreateExport(ctx, req); err != nil {
		return nil, err
	}
	if err := s.audit(ctx, "privacy.export.requested", "contact", contactID, map[string]any{"request_id": req.ID}); err != nil {
		return nil, err
	}
	if err := s.enqueuer.EnqueueExport(contracts.ExportTask{TenantID: tenantID, RequestID: req.ID, ActorID: req.RequestedBy}); err != nil {
		return nil, err
	}
	return req, nil
}

// RunExport assembles the contact's chat-data bundle, writes it to the file store
// and attaches a temporary signed URL to the request. Idempotent: a request that
// is already ready is a no-op. Only chat data is included — never external
// provider data, which is not persisted.
func (s *Service) RunExport(ctx context.Context, requestID string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	req, err := s.store.FindExport(ctx, requestID)
	if err != nil {
		return err
	}
	if req.Status == entity.ExportReady {
		return nil
	}
	bundle, err := s.store.CollectBundle(ctx, req.ContactID)
	if err != nil {
		return s.failExport(ctx, req, err)
	}
	bundle.GeneratedAt = s.clock.Now()
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return s.failExport(ctx, req, err)
	}
	key := exportKey(req.TenantID, req.ContactID, req.ID)
	if err := s.files.Save(key, data); err != nil {
		return s.failExport(ctx, req, err)
	}
	url, expiresAt, err := s.files.SignedURL(key, s.urlTTL)
	if err != nil {
		return s.failExport(ctx, req, err)
	}
	now := s.clock.Now()
	req.Status = entity.ExportReady
	req.StorageKey = key
	req.DownloadURL = url
	req.ExpiresAt = expiresAt
	req.CompletedAt = &now
	req.Error = ""
	if err := s.store.UpdateExport(ctx, req); err != nil {
		return err
	}
	return s.audit(ctx, "privacy.export.generated", "contact", req.ContactID, map[string]any{
		"request_id":    req.ID,
		"conversations": len(bundle.Conversations),
		"bytes":         len(data),
		"expires_at":    expiresAt,
	})
}

func (s *Service) failExport(ctx context.Context, req *entity.ExportRequest, cause error) error {
	req.Status = entity.ExportFailed
	req.Error = cause.Error()
	if uerr := s.store.UpdateExport(ctx, req); uerr != nil {
		return uerr
	}
	_ = s.audit(ctx, "privacy.export.failed", "contact", req.ContactID, map[string]any{
		"request_id": req.ID, "error": cause.Error(),
	})
	return cause
}

// GetExport returns an export request (e.g. to poll for the signed URL).
func (s *Service) GetExport(ctx context.Context, id string) (*entity.ExportRequest, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.store.FindExport(ctx, id)
}

// Anonymize replaces the contact's PII with anonymized values and masks PII in
// the contact's messages, keeping referential integrity and aggregate metrics
// (the contact row and id are retained). It refuses contacts under an active
// legal hold.
func (s *Service) Anonymize(ctx context.Context, contactID string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	contactID = strings.TrimSpace(contactID)
	if contactID == "" {
		return apperror.Validation("contact id is required")
	}
	now := s.clock.Now()
	held, err := s.store.HasActiveLegalHold(ctx, contactID, now)
	if err != nil {
		return err
	}
	if held {
		_ = s.audit(ctx, "privacy.contact.anonymize_refused", "contact", contactID, map[string]any{"reason": "legal_hold"})
		return apperror.Forbidden("contact is under a legal hold and cannot be anonymized before the deadline")
	}

	// Collect the bundle first: it 404s a missing contact and gives us the
	// original PII (needed to scrub literals from message text) plus the message
	// ids to mask.
	bundle, err := s.store.CollectBundle(ctx, contactID)
	if err != nil {
		return err
	}

	// Idempotent: a contact already anonymized is a no-op success. Re-processing
	// after the commit point has nothing to scrub (the original PII is gone) and
	// must never 500.
	if bundle.Contact.Anonymized {
		return s.audit(ctx, "privacy.contact.anonymize_noop", "contact", contactID, map[string]any{"reason": "already_anonymized"})
	}

	pii := piiOf(bundle.Contact)

	// Order matters for crash-safety WITHOUT a transaction: mask the messages
	// FIRST (using the original PII), then flip the contact's anonymized flag LAST
	// as the commit point. Each step is idempotent (re-masking already-masked text
	// is a no-op; the $set writes the same values), so a partial failure is
	// recovered by a retry — the contact is still un-flagged, the bundle still
	// carries the original PII, and re-masking is harmless. Conversations and
	// messages are never deleted; only the contact PII literals are scrubbed.
	masked := 0
	for _, conv := range bundle.Conversations {
		for _, m := range conv.Messages {
			nt := maskPII(m.Text, pii)
			if nt != m.Text {
				if err := s.store.UpdateMessageText(ctx, m.ID, nt); err != nil {
					return err
				}
				masked++
			}
		}
	}

	if err := s.store.AnonymizeContact(ctx, contactID, repository.Anonymized{
		Name:     anonymizedName,
		Phone:    "",
		Document: "",
	}); err != nil {
		return err
	}

	return s.audit(ctx, "privacy.contact.anonymized", "contact", contactID, map[string]any{
		"messages_masked": masked,
		"conversations":   len(bundle.Conversations),
	})
}

// GetRetention returns the tenant's retention policy, or an all-zero
// (keep-forever) default when none is configured.
func (s *Service) GetRetention(ctx context.Context) (*entity.RetentionPolicy, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.store.GetRetention(ctx)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return entity.DefaultRetention(tenantID), nil
	}
	return p, nil
}

// UpdateRetention applies a partial update to the tenant's retention policy.
func (s *Service) UpdateRetention(ctx context.Context, cmd contracts.UpdateRetention) (*entity.RetentionPolicy, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	p, err := s.GetRetention(ctx)
	if err != nil {
		return nil, err
	}
	if cmd.MessagesDays != nil {
		p.MessagesDays = *cmd.MessagesDays
	}
	if cmd.ClosedConversationsDays != nil {
		p.ClosedConversationsDays = *cmd.ClosedConversationsDays
	}
	if cmd.TechnicalLogsDays != nil {
		p.TechnicalLogsDays = *cmd.TechnicalLogsDays
	}
	if cmd.AuditLogsDays != nil {
		p.AuditLogsDays = *cmd.AuditLogsDays
	}
	if cmd.NotificationsDays != nil {
		p.NotificationsDays = *cmd.NotificationsDays
	}
	p.Normalize()
	p.UpdatedAt = s.clock.Now()
	if err := s.store.SaveRetention(ctx, p); err != nil {
		return nil, err
	}
	if err := s.audit(ctx, "privacy.retention.updated", "retention_policy", p.TenantID, map[string]any{
		"messages_days":             p.MessagesDays,
		"closed_conversations_days": p.ClosedConversationsDays,
		"technical_logs_days":       p.TechnicalLogsDays,
		"audit_logs_days":           p.AuditLogsDays,
		"notifications_days":        p.NotificationsDays,
	}); err != nil {
		return nil, err
	}
	return p, nil
}

// ApplyRetention enforces the tenant's retention policy, deleting data past each
// cutoff while skipping anything under an active legal hold. Returns the total
// number of documents removed. Called by the privacy.retention scheduler job
// (fanned out across tenants by the worker).
func (s *Service) ApplyRetention(ctx context.Context) (int, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return 0, err
	}
	p, err := s.store.GetRetention(ctx)
	if err != nil {
		return 0, err
	}
	if p == nil {
		return 0, nil // nothing configured → nothing to do
	}
	res, err := s.store.ApplyRetention(ctx, *p, s.clock.Now())
	if err != nil {
		return 0, err
	}
	if res.Total() > 0 {
		if err := s.audit(ctx, "privacy.retention.applied", "retention_policy", p.TenantID, map[string]any{
			"messages":             res.Messages,
			"closed_conversations": res.ClosedConversations,
			"technical_logs":       res.TechnicalLogs,
			"audit_logs":           res.AuditLogs,
			"notifications":        res.Notifications,
		}); err != nil {
			return res.Total(), err
		}
	}
	return res.Total(), nil
}

// audit records an audit entry; a failed audit fails the operation.
func (s *Service) audit(ctx context.Context, action, resourceType, resourceID string, meta map[string]any) error {
	return s.auditor.Record(ctx, shared.AuditEntry{
		ActorID:      actorID(ctx),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Data:         meta,
		At:           s.clock.Now(),
	})
}

func exportKey(tenantID, contactID, requestID string) string {
	return fmt.Sprintf("exports/%s/%s/%s.json", tenantID, contactID, requestID)
}

// actorID resolves the acting principal for audit: the authenticated user, or
// "system" for internal jobs.
func actorID(ctx context.Context) string {
	if ac, ok := authz.FromContext(ctx); ok && ac.UserID != "" {
		return ac.UserID
	}
	return "system"
}
