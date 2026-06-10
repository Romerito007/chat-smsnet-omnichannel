// Package service holds the SLA business logic: policy CRUD, per-conversation
// tracking (lifecycle hook), the at-risk listing and the scheduler breach check.
package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sla/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sla/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sla/repository"
)

// PolicyService manages SLA policies.
type PolicyService struct {
	repo  repository.PolicyRepository
	clock shared.Clock
}

// NewPolicyService builds the service.
func NewPolicyService(repo repository.PolicyRepository, clock shared.Clock) *PolicyService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &PolicyService{repo: repo, clock: clock}
}

// Create creates a policy.
func (s *PolicyService) Create(ctx context.Context, cmd contracts.CreatePolicy) (*entity.SLAPolicy, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.Name) == "" {
		return nil, apperror.Validation("name is required").WithDetails(map[string]any{"name": "is required"})
	}
	if cmd.FirstResponseTargetSec <= 0 && cmd.ResolutionTargetSec <= 0 {
		return nil, apperror.Validation("at least one target must be set").
			WithDetails(map[string]any{"targets": "first_response or resolution required"})
	}
	warn := cmd.WarningThresholdPct
	if warn <= 0 || warn >= 100 {
		warn = 80 // sensible default
	}
	now := s.clock.Now()
	p := &entity.SLAPolicy{
		ID:                     shared.NewID(),
		TenantID:               tenantID,
		Name:                   strings.TrimSpace(cmd.Name),
		SectorIDs:              cmd.SectorIDs,
		Priority:               strings.TrimSpace(cmd.Priority),
		Channel:                strings.TrimSpace(cmd.Channel),
		FirstResponseTargetSec: cmd.FirstResponseTargetSec,
		ResolutionTargetSec:    cmd.ResolutionTargetSec,
		BusinessHoursOnly:      cmd.BusinessHoursOnly,
		WarningThresholdPct:    warn,
		PauseOnWaitingCustomer: cmd.PauseOnWaitingCustomer,
		Enabled:                cmd.Enabled == nil || *cmd.Enabled,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Update patches a policy.
func (s *PolicyService) Update(ctx context.Context, id string, cmd contracts.UpdatePolicy) (*entity.SLAPolicy, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		if strings.TrimSpace(*cmd.Name) == "" {
			return nil, apperror.Validation("name cannot be empty").WithDetails(map[string]any{"name": "cannot be empty"})
		}
		p.Name = strings.TrimSpace(*cmd.Name)
	}
	if cmd.SectorIDs != nil {
		p.SectorIDs = *cmd.SectorIDs
	}
	if cmd.Priority != nil {
		p.Priority = strings.TrimSpace(*cmd.Priority)
	}
	if cmd.Channel != nil {
		p.Channel = strings.TrimSpace(*cmd.Channel)
	}
	if cmd.FirstResponseTargetSec != nil {
		p.FirstResponseTargetSec = *cmd.FirstResponseTargetSec
	}
	if cmd.ResolutionTargetSec != nil {
		p.ResolutionTargetSec = *cmd.ResolutionTargetSec
	}
	if cmd.BusinessHoursOnly != nil {
		p.BusinessHoursOnly = *cmd.BusinessHoursOnly
	}
	if cmd.WarningThresholdPct != nil && *cmd.WarningThresholdPct > 0 && *cmd.WarningThresholdPct < 100 {
		p.WarningThresholdPct = *cmd.WarningThresholdPct
	}
	if cmd.PauseOnWaitingCustomer != nil {
		p.PauseOnWaitingCustomer = *cmd.PauseOnWaitingCustomer
	}
	if cmd.Enabled != nil {
		p.Enabled = *cmd.Enabled
	}
	p.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete removes a policy.
func (s *PolicyService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Get returns a policy.
func (s *PolicyService) Get(ctx context.Context, id string) (*entity.SLAPolicy, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns the tenant's policies.
func (s *PolicyService) List(ctx context.Context, page shared.PageRequest) ([]*entity.SLAPolicy, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, page.Normalize())
}
