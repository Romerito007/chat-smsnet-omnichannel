package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// CloseReasonService manages close reasons and answers the conversations
// domain's requires_note policy.
type CloseReasonService struct {
	repo  repository.CloseReasonRepository
	clock shared.Clock
}

// NewCloseReasonService builds the service.
func NewCloseReasonService(repo repository.CloseReasonRepository, clock shared.Clock) *CloseReasonService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &CloseReasonService{repo: repo, clock: clock}
}

// Create creates a close reason.
func (s *CloseReasonService) Create(ctx context.Context, cmd contracts.CreateCloseReason) (*entity.CloseReason, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.Name) == "" {
		return nil, apperror.Validation("name is required").WithDetails(map[string]any{"name": "is required"})
	}
	now := s.clock.Now()
	c := &entity.CloseReason{
		ID:           shared.NewID(),
		TenantID:     tenantID,
		Name:         strings.TrimSpace(cmd.Name),
		RequiresNote: boolOr(cmd.RequiresNote, false),
		Enabled:      boolOr(cmd.Enabled, true),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Update patches a close reason.
func (s *CloseReasonService) Update(ctx context.Context, id string, cmd contracts.UpdateCloseReason) (*entity.CloseReason, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	c, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		if strings.TrimSpace(*cmd.Name) == "" {
			return nil, apperror.Validation("name cannot be empty").WithDetails(map[string]any{"name": "cannot be empty"})
		}
		c.Name = strings.TrimSpace(*cmd.Name)
	}
	if cmd.RequiresNote != nil {
		c.RequiresNote = *cmd.RequiresNote
	}
	if cmd.Enabled != nil {
		c.Enabled = *cmd.Enabled
	}
	c.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Delete removes a close reason.
func (s *CloseReasonService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Get returns a close reason.
func (s *CloseReasonService) Get(ctx context.Context, id string) (*entity.CloseReason, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns the tenant's close reasons.
func (s *CloseReasonService) List(ctx context.Context, page shared.PageRequest) ([]*entity.CloseReason, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, page.Normalize())
}

// Names resolves a set of close-reason ids to their names in ONE batch lookup, for
// enriching the conversations report (closed_by_reason). Missing ids are absent.
func (s *CloseReasonService) Names(ctx context.Context, ids []string) (map[string]string, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	reasons, err := s.repo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(reasons))
	for _, rsn := range reasons {
		out[rsn.ID] = rsn.Name
	}
	return out, nil
}

// RequiresNote implements conversations' CloseReasonPolicy. An unknown reason
// yields a not_found error; a disabled reason is treated as not requiring a note
// (it should not be offered, but must not block closing).
func (s *CloseReasonService) RequiresNote(ctx context.Context, reasonID string) (bool, error) {
	c, err := s.repo.FindByID(ctx, reasonID)
	if err != nil {
		return false, err
	}
	return c.RequiresNote, nil
}

var _ convcontracts.CloseReasonPolicy = (*CloseReasonService)(nil)
