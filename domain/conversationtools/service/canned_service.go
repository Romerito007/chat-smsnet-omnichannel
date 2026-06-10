package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// CannedResponseService manages canned responses and resolves shortcuts.
type CannedResponseService struct {
	repo  repository.CannedResponseRepository
	clock shared.Clock
}

// NewCannedResponseService builds the service.
func NewCannedResponseService(repo repository.CannedResponseRepository, clock shared.Clock) *CannedResponseService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &CannedResponseService{repo: repo, clock: clock}
}

// Create creates a canned response.
func (s *CannedResponseService) Create(ctx context.Context, cmd contracts.CreateCannedResponse) (*entity.CannedResponse, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	shortcut := normalizeShortcut(cmd.Shortcut)
	if shortcut == "" {
		return nil, apperror.Validation("shortcut is required").WithDetails(map[string]any{"shortcut": "is required"})
	}
	if strings.TrimSpace(cmd.Body) == "" {
		return nil, apperror.Validation("body is required").WithDetails(map[string]any{"body": "is required"})
	}
	if existing, err := s.repo.FindByShortcut(ctx, shortcut); err == nil && existing != nil {
		return nil, apperror.Conflict("a canned response with this shortcut already exists")
	} else if err != nil && apperror.From(err).Code != apperror.CodeNotFound {
		return nil, err
	}

	now := s.clock.Now()
	c := &entity.CannedResponse{
		ID:        shared.NewID(),
		TenantID:  tenantID,
		SectorIDs: cmd.SectorIDs,
		Shortcut:  shortcut,
		Title:     strings.TrimSpace(cmd.Title),
		Body:      cmd.Body,
		Enabled:   boolOr(cmd.Enabled, true),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Update patches a canned response.
func (s *CannedResponseService) Update(ctx context.Context, id string, cmd contracts.UpdateCannedResponse) (*entity.CannedResponse, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	c, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Shortcut != nil {
		shortcut := normalizeShortcut(*cmd.Shortcut)
		if shortcut == "" {
			return nil, apperror.Validation("shortcut cannot be empty").WithDetails(map[string]any{"shortcut": "cannot be empty"})
		}
		if existing, ferr := s.repo.FindByShortcut(ctx, shortcut); ferr == nil && existing != nil && existing.ID != c.ID {
			return nil, apperror.Conflict("a canned response with this shortcut already exists")
		} else if ferr != nil && apperror.From(ferr).Code != apperror.CodeNotFound {
			return nil, ferr
		}
		c.Shortcut = shortcut
	}
	if cmd.SectorIDs != nil {
		c.SectorIDs = *cmd.SectorIDs
	}
	if cmd.Title != nil {
		c.Title = strings.TrimSpace(*cmd.Title)
	}
	if cmd.Body != nil {
		if strings.TrimSpace(*cmd.Body) == "" {
			return nil, apperror.Validation("body cannot be empty").WithDetails(map[string]any{"body": "cannot be empty"})
		}
		c.Body = *cmd.Body
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

// Delete removes a canned response.
func (s *CannedResponseService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Get returns a canned response.
func (s *CannedResponseService) Get(ctx context.Context, id string) (*entity.CannedResponse, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns the tenant's canned responses.
func (s *CannedResponseService) List(ctx context.Context, page shared.PageRequest) ([]*entity.CannedResponse, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, page.Normalize())
}

// Resolve returns the body for a shortcut, enforcing that the response is enabled
// and visible to the actor's sectors. This is the agent-facing "type a shortcut,
// get the body" lookup.
func (s *CannedResponseService) Resolve(ctx context.Context, shortcut string) (*entity.CannedResponse, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	c, err := s.repo.FindByShortcut(ctx, normalizeShortcut(shortcut))
	if err != nil {
		return nil, err
	}
	if !c.Enabled {
		return nil, apperror.NotFound("canned response not found")
	}
	// Sector restriction: full-scope actors see everything; others must share a
	// sector with the response (or it must be global).
	if ac, ok := authz.FromContext(ctx); ok && ac.SectorScope != authz.ScopeAll {
		if !c.VisibleToSectors(ac.SectorIDs) {
			return nil, apperror.NotFound("canned response not found")
		}
	}
	return c, nil
}

// normalizeShortcut trims and lowercases the shortcut, stripping a leading '/'.
func normalizeShortcut(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	return strings.TrimPrefix(s, "/")
}
