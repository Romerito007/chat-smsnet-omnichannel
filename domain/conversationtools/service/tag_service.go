// Package service holds the conversationtools business logic: CRUD for tags,
// canned responses and close reasons, plus the ports the conversations domain
// uses to validate tags and enforce close-reason notes.
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

// TagService manages tags and validates tag ids for the conversations domain.
type TagService struct {
	repo  repository.TagRepository
	clock shared.Clock
}

// NewTagService builds the service.
func NewTagService(repo repository.TagRepository, clock shared.Clock) *TagService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &TagService{repo: repo, clock: clock}
}

// Create creates a tag.
func (s *TagService) Create(ctx context.Context, cmd contracts.CreateTag) (*entity.Tag, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.Name) == "" {
		return nil, apperror.Validation("name is required").WithDetails(map[string]any{"name": "is required"})
	}
	now := s.clock.Now()
	t := &entity.Tag{
		ID:          shared.NewID(),
		TenantID:    tenantID,
		Name:        strings.TrimSpace(cmd.Name),
		Color:       strings.TrimSpace(cmd.Color),
		Description: strings.TrimSpace(cmd.Description),
		Enabled:     boolOr(cmd.Enabled, true),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Update patches a tag.
func (s *TagService) Update(ctx context.Context, id string, cmd contracts.UpdateTag) (*entity.Tag, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	t, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		if strings.TrimSpace(*cmd.Name) == "" {
			return nil, apperror.Validation("name cannot be empty").WithDetails(map[string]any{"name": "cannot be empty"})
		}
		t.Name = strings.TrimSpace(*cmd.Name)
	}
	if cmd.Color != nil {
		t.Color = strings.TrimSpace(*cmd.Color)
	}
	if cmd.Description != nil {
		t.Description = strings.TrimSpace(*cmd.Description)
	}
	if cmd.Enabled != nil {
		t.Enabled = *cmd.Enabled
	}
	t.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Delete removes a tag.
func (s *TagService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Get returns a tag.
func (s *TagService) Get(ctx context.Context, id string) (*entity.Tag, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns the tenant's tags.
func (s *TagService) List(ctx context.Context, page shared.PageRequest) ([]*entity.Tag, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, page.Normalize())
}

// ValidateTags implements conversations' TagCatalog: every id must exist, belong
// to the tenant and be enabled.
func (s *TagService) ValidateTags(ctx context.Context, tagIDs []string) error {
	if len(tagIDs) == 0 {
		return nil
	}
	found, err := s.repo.FindByIDs(ctx, tagIDs)
	if err != nil {
		return err
	}
	byID := make(map[string]*entity.Tag, len(found))
	for _, t := range found {
		byID[t.ID] = t
	}
	for _, id := range tagIDs {
		t, ok := byID[id]
		if !ok {
			return apperror.Validation("unknown tag: " + id).WithDetails(map[string]any{"tags": "unknown tag " + id})
		}
		if !t.Enabled {
			return apperror.Validation("tag is disabled: " + id).WithDetails(map[string]any{"tags": "disabled tag " + id})
		}
	}
	return nil
}

// TagCard is a tag's display data (name + color) for batch chip rendering.
type TagCard struct {
	Name  string
	Color string
}

// Cards resolves tag ids to their display cards (name + color) in ONE call, so a
// consumer (e.g. the deals card) can render coloured chips without fetching each tag.
func (s *TagService) Cards(ctx context.Context, ids []string) (map[string]TagCard, error) {
	if len(ids) == 0 {
		return map[string]TagCard{}, nil
	}
	found, err := s.repo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]TagCard, len(found))
	for _, t := range found {
		out[t.ID] = TagCard{Name: t.Name, Color: t.Color}
	}
	return out, nil
}

// ResolveTags maps each ref (a tag id OR a tag name) to its canonical id so a
// tags array is always stored as ids. strict rejects unknown/disabled refs (add);
// lenient passes unresolved refs through (remove). De-duplicates the result.
func (s *TagService) ResolveTags(ctx context.Context, refs []string, strict bool) ([]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	tags, err := s.repo.List(ctx, shared.PageRequest{Limit: shared.MaxPageSize})
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*entity.Tag, len(tags))
	byName := make(map[string]*entity.Tag, len(tags))
	for _, t := range tags {
		byID[t.ID] = t
		byName[t.Name] = t
	}

	out := make([]string, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		id := ref
		var tag *entity.Tag
		if t, ok := byID[ref]; ok {
			tag = t
		} else if t, ok := byName[ref]; ok {
			id, tag = t.ID, t
		} else if strict {
			return nil, apperror.Validation("tag not found: " + ref).
				WithDetails(map[string]any{"tags": "unknown tag " + ref})
		}
		if strict && tag != nil && !tag.Enabled {
			return nil, apperror.Validation("tag is disabled: " + ref).
				WithDetails(map[string]any{"tags": "disabled tag " + ref})
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func boolOr(p *bool, def bool) bool {
	if p != nil {
		return *p
	}
	return def
}

var _ convcontracts.TagCatalog = (*TagService)(nil)
