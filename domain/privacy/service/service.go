// Package service implements the privacy (LGPD) domain: contact data export,
// erasure (right to be forgotten) and per-tenant retention. Every action is
// audited ("toda ação auditada") and erasure refuses contacts under an active
// legal hold ("não apagar dados sob obrigação legal antes do prazo").
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
	blobs    contracts.BlobStore
	enqueuer contracts.ExportEnqueuer
	auditor  shared.Auditor
	clock    shared.Clock
	urlTTL   time.Duration
}

// NewService builds the service. A nil auditor/clock falls back to safe
// defaults; urlTTL <= 0 uses the 24h default. blobs may be nil (export-only
// deployments) — erasure then skips attachment-blob purging.
func NewService(store repository.Store, files contracts.FileStore, blobs contracts.BlobStore, enqueuer contracts.ExportEnqueuer, auditor shared.Auditor, clock shared.Clock, urlTTL time.Duration) *Service {
	if auditor == nil {
		auditor = shared.NoopAuditor{}
	}
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if urlTTL <= 0 {
		urlTTL = defaultURLTTL
	}
	return &Service{store: store, files: files, blobs: blobs, enqueuer: enqueuer, auditor: auditor, clock: clock, urlTTL: urlTTL}
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

// EraseContact hard-deletes a contact and all of its personal data /
// communications (right to be forgotten), unlinking — but never deleting — any
// CRM deal so the company keeps its commercial record. It refuses contacts under
// an active legal hold.
//
// Deal warning flow: when the contact still has linked deals and force is false,
// it returns a 409 conflict carrying the deal list and erases nothing — the
// company is expected to review the link first. Calling again with force=true
// severs the deals and proceeds.
func (s *Service) EraseContact(ctx context.Context, contactID string, force bool) error {
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
		_ = s.audit(ctx, "privacy.contact.erase_refused", "contact", contactID, map[string]any{"reason": "legal_hold"})
		return apperror.Forbidden("contact is under a legal hold and cannot be erased before the deadline")
	}

	// Warn before destroying: a contact with linked deals is blocked until the
	// caller confirms with force=true. The deal ids/titles are returned so the
	// company can treat the link in the CRM first.
	deals, err := s.store.LinkedDeals(ctx, contactID)
	if err != nil {
		return err
	}
	if len(deals) > 0 && !force {
		_ = s.audit(ctx, "privacy.contact.erase_blocked", "contact", contactID, map[string]any{
			"reason": "linked_deals", "deals": len(deals),
		})
		return apperror.Conflict("contact has linked deals; review the link and retry with force=true to unlink and erase").
			WithDetails(map[string]any{"linked_deals": deals})
	}

	// Destructive cascade. The repo collects the physical storage keys before
	// removing the rows and returns them so we can purge the blobs/export files
	// afterwards. With force, it also severs (not deletes) the linked deals.
	res, err := s.store.EraseContact(ctx, contactID, force)
	if err != nil {
		return err
	}

	// Best-effort physical purge of media blobs and export bundles. These are not
	// transactional with the DB; a failure here leaves an orphan file but never a
	// dangling DB row. We log-and-continue rather than fail the erasure, which has
	// already removed every personal-data row.
	//
	// TODO(privacy): because this purge is best-effort, a failed Delete leaves an
	// owner-less blob behind. Add a periodic garbage-collection job that reconciles
	// the attachments/export storage against the DB and removes orphaned objects so
	// they do not accumulate over time.
	blobsDeleted := 0
	if s.blobs != nil {
		for _, key := range res.BlobKeys {
			if derr := s.blobs.Delete(key); derr == nil {
				blobsDeleted++
			}
		}
	}
	exportsDeleted := 0
	for _, key := range res.ExportKeys {
		if derr := s.files.Delete(key); derr == nil {
			exportsDeleted++
		}
	}

	return s.audit(ctx, "privacy.contact.erased", "contact", contactID, map[string]any{
		"conversations":   res.Conversations,
		"messages":        res.Messages,
		"documents":       res.Documents(),
		"deals_unlinked":  res.DealsUnlinked,
		"blobs_deleted":   blobsDeleted,
		"exports_deleted": exportsDeleted,
		"forced":          force,
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
	// Best-effort purge of the media blobs whose attachments were cascade-deleted
	// alongside retired conversations — closing the orphan-blob leak.
	blobsDeleted := 0
	if s.blobs != nil {
		for _, key := range res.BlobKeys {
			if derr := s.blobs.Delete(key); derr == nil {
				blobsDeleted++
			}
		}
	}
	if res.Total() > 0 {
		if err := s.audit(ctx, "privacy.retention.applied", "retention_policy", p.TenantID, map[string]any{
			"messages":             res.Messages,
			"closed_conversations": res.ClosedConversations,
			"satellite_docs":       res.SatelliteDocs,
			"technical_logs":       res.TechnicalLogs,
			"audit_logs":           res.AuditLogs,
			"notifications":        res.Notifications,
			"blobs_deleted":        blobsDeleted,
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
