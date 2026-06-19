// Package service holds the CRM-settings business logic: read the tenant's module
// toggles (with conservative defaults when never configured) and update them. It is
// the single checkpoint the optional CRM modules (tasks, products, timeline) consult
// via IsModuleEnabled before serving their endpoints.
package service

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service manages a tenant's CRM settings.
type Service struct {
	repo    repository.CRMSettingsRepository
	auditor shared.Auditor
	clock   shared.Clock
}

// New builds the service.
func New(repo repository.CRMSettingsRepository, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, auditor: shared.NoopAuditor{}, clock: clock}
}

// SetAuditor wires the audit trail (optional).
func (s *Service) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// Get returns the tenant's CRM settings, falling back to the conservative defaults
// (timeline on, tasks/products off) when the tenant has never configured them. The
// defaults are NOT persisted — a read has no side effects.
func (s *Service) Get(ctx context.Context) (*entity.CRMSettings, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := s.repo.Get(ctx)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return entity.Default(tenantID), nil
		}
		return nil, err
	}
	return cur, nil
}

// Update applies the non-nil toggles over the current settings (or the defaults when
// never configured) and persists them.
func (s *Service) Update(ctx context.Context, cmd contracts.UpdateCRMSettings) (*entity.CRMSettings, error) {
	cur, err := s.Get(ctx)
	if err != nil {
		return nil, err
	}
	if cmd.TasksEnabled != nil {
		cur.TasksEnabled = *cmd.TasksEnabled
	}
	if cmd.ProductsEnabled != nil {
		cur.ProductsEnabled = *cmd.ProductsEnabled
	}
	if cmd.TimelineEnabled != nil {
		cur.TimelineEnabled = *cmd.TimelineEnabled
	}
	cur.UpdatedAt = s.clock.Now()
	if err := s.repo.Upsert(ctx, cur); err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "crm.settings_updated", ResourceType: "crm_settings", ResourceID: cur.TenantID,
		Data: map[string]any{
			"tasks_enabled": cur.TasksEnabled, "products_enabled": cur.ProductsEnabled,
			"timeline_enabled": cur.TimelineEnabled,
		},
	})
	return cur, nil
}

// IsModuleEnabled reports whether an optional CRM module (tasks|products|timeline) is
// enabled for the tenant. This is the checkpoint the future module endpoints call
// before serving — a disabled module returns empty/disabled and the front hides it.
func (s *Service) IsModuleEnabled(ctx context.Context, module entity.Module) (bool, error) {
	st, err := s.Get(ctx)
	if err != nil {
		return false, err
	}
	return st.Enabled(module), nil
}
