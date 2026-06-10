// Package service holds the Tenant business logic.
package service

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/repository"
)

// Service exposes tenant read/update operations. The tenant id always comes
// from the authenticated context — never from the client.
type Service struct {
	repo  repository.TenantRepository
	clock shared.Clock
}

// New builds the tenant service.
func New(repo repository.TenantRepository, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, clock: clock}
}

// Current returns the tenant for the authenticated scope on the context.
func (s *Service) Current(ctx context.Context) (*entity.Tenant, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, tenantID)
}

// UpdateSettings replaces the tenant settings and name for the current tenant.
func (s *Service) UpdateSettings(ctx context.Context, name string, settings map[string]any) (*entity.Tenant, error) {
	t, err := s.Current(ctx)
	if err != nil {
		return nil, err
	}
	if name != "" {
		t.Name = name
	}
	if settings != nil {
		t.Settings = settings
	}
	if t.Status == "" {
		return nil, apperror.Internal("tenant has no status")
	}
	t.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}
