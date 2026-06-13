package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/repository"
	phrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// CreateAssistant is the input to AssistantService.Create.
type CreateAssistant struct {
	Name         string
	ChannelTypes []string
	ISPProfileID string
	Enabled      *bool // nil → true
}

// UpdateAssistant carries optional fields; nil pointers mean "leave unchanged".
type UpdateAssistant struct {
	Name         *string
	ChannelTypes *[]string
	ISPProfileID *string
	Enabled      *bool
}

// AssistantService manages copilot assistants (many per tenant). It validates that
// a pinned ISP profile exists, and answers whether a profile is in use (so
// providerhub can block deleting a referenced profile).
type AssistantService struct {
	repo     repository.AssistantRepository
	profiles phrepo.ProfileRepository
	clock    shared.Clock
}

// NewAssistantService builds the service.
func NewAssistantService(repo repository.AssistantRepository, profiles phrepo.ProfileRepository, clock shared.Clock) *AssistantService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &AssistantService{repo: repo, profiles: profiles, clock: clock}
}

// List returns the tenant's assistants.
func (s *AssistantService) List(ctx context.Context) ([]*entity.Assistant, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

// Get returns one assistant by id.
func (s *AssistantService) Get(ctx context.Context, id string) (*entity.Assistant, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// Create registers a new assistant.
func (s *AssistantService) Create(ctx context.Context, cmd CreateAssistant) (*entity.Assistant, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, apperror.Validation("name is required")
	}
	ispProfileID := strings.TrimSpace(cmd.ISPProfileID)
	if err := s.validateProfile(ctx, ispProfileID); err != nil {
		return nil, err
	}
	enabled := true
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	}
	now := s.clock.Now()
	a := &entity.Assistant{
		ID:           shared.NewID(),
		TenantID:     tenantID,
		Name:         name,
		ChannelTypes: cmd.ChannelTypes,
		ISPProfileID: ispProfileID,
		Enabled:      enabled,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Update applies the non-nil fields of cmd.
func (s *AssistantService) Update(ctx context.Context, id string, cmd UpdateAssistant) (*entity.Assistant, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		a.Name = strings.TrimSpace(*cmd.Name)
		if a.Name == "" {
			return nil, apperror.Validation("name is required")
		}
	}
	if cmd.ChannelTypes != nil {
		a.ChannelTypes = *cmd.ChannelTypes
	}
	if cmd.ISPProfileID != nil {
		a.ISPProfileID = strings.TrimSpace(*cmd.ISPProfileID)
		if err := s.validateProfile(ctx, a.ISPProfileID); err != nil {
			return nil, err
		}
	}
	if cmd.Enabled != nil {
		a.Enabled = *cmd.Enabled
	}
	a.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Delete removes an assistant.
func (s *AssistantService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// IsISPProfileInUse reports whether any assistant references the ISP profile, and
// the name of one such assistant (for a clear "in use" message). Implements the
// providerhub ProfileUsageChecker port.
func (s *AssistantService) IsISPProfileInUse(ctx context.Context, ispProfileID string) (bool, string, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return false, "", err
	}
	all, err := s.repo.List(ctx)
	if err != nil {
		return false, "", err
	}
	for _, a := range all {
		if a.ISPProfileID == ispProfileID {
			return true, a.Name, nil
		}
	}
	return false, "", nil
}

// validateProfile ensures a pinned ISP profile exists for the tenant (empty = no
// ISP, allowed).
func (s *AssistantService) validateProfile(ctx context.Context, ispProfileID string) error {
	if ispProfileID == "" {
		return nil
	}
	if _, err := s.profiles.FindByID(ctx, ispProfileID); err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return apperror.Validation("unknown isp_profile_id")
		}
		return err
	}
	return nil
}
