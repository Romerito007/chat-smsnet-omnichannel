// Package service implements the audit domain: it persists audit entries
// (implementing the shared.Auditor port consulted by other domains) and serves
// the audit-log query used by the audit.view endpoint.
package service

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/audit/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/audit/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service writes and reads audit logs.
type Service struct {
	repo  repository.Repository
	clock shared.Clock
}

// NewService builds the service.
func NewService(repo repository.Repository, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, clock: clock}
}

// Record implements shared.Auditor. The tenant is taken from the entry (the
// producer reads it from context); the repository scopes the write by it.
func (s *Service) Record(ctx context.Context, e shared.AuditEntry) error {
	if e.TenantID == "" {
		// Fall back to the request tenant so callers can omit it.
		tenantID, err := shared.RequireTenant(ctx)
		if err != nil {
			return err
		}
		e.TenantID = tenantID
	}
	at := e.At
	if at.IsZero() {
		at = s.clock.Now()
	}
	log := &entity.AuditLog{
		ID:           shared.NewID(),
		TenantID:     e.TenantID,
		ActorID:      e.ActorID,
		Action:       e.Action,
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		Metadata:     e.Metadata,
		CreatedAt:    at,
	}
	return s.repo.Create(ctx, log)
}

// List returns the tenant's audit logs (newest first).
func (s *Service) List(ctx context.Context, f repository.Filter, page shared.PageRequest) ([]*entity.AuditLog, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, f, page)
}

var _ shared.Auditor = (*Service)(nil)
