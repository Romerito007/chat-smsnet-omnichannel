// Package service implements the audit domain: it persists audit entries
// (implementing the shared.Auditor port consulted by other domains) and serves
// the audit-log query used by the audit.view endpoint.
package service

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/audit/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/audit/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
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

// Record implements shared.Auditor. Tenant, actor (id + type), client ip and
// user-agent are filled from the request context when the producer leaves them
// empty, so callers deep in the service layer can audit with a one-liner.
func (s *Service) Record(ctx context.Context, e shared.AuditEntry) error {
	if e.TenantID == "" {
		tenantID, err := shared.RequireTenant(ctx)
		if err != nil {
			return err
		}
		e.TenantID = tenantID
	}
	s.fillActor(ctx, &e)
	if ip, ua := shared.AuditMetaFrom(ctx); true {
		if e.IP == "" {
			e.IP = ip
		}
		if e.UserAgent == "" {
			e.UserAgent = ua
		}
	}
	at := e.At
	if at.IsZero() {
		at = s.clock.Now()
	}
	log := &entity.AuditLog{
		ID:           shared.NewID(),
		TenantID:     e.TenantID,
		ActorID:      e.ActorID,
		ActorType:    e.ActorType,
		Action:       e.Action,
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		IP:           e.IP,
		UserAgent:    e.UserAgent,
		Data:         e.Data,
		CreatedAt:    at,
	}
	return s.repo.Create(ctx, log)
}

// fillActor derives the actor id and type from the auth context when unset.
func (s *Service) fillActor(ctx context.Context, e *shared.AuditEntry) {
	if ac, ok := authz.FromContext(ctx); ok {
		if e.ActorID == "" {
			e.ActorID = ac.UserID
		}
		if e.ActorType == "" {
			if ac.UserID == "system" {
				e.ActorType = shared.ActorTypeSystem
			} else {
				e.ActorType = shared.ActorTypeUser
			}
		}
	}
	if e.ActorType == "" {
		e.ActorType = shared.ActorTypePublic
	}
}

// List returns the tenant's audit logs (newest first).
func (s *Service) List(ctx context.Context, f repository.Filter, page shared.PageRequest) ([]*entity.AuditLog, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, f, page)
}

var _ shared.Auditor = (*Service)(nil)
