// Package service holds the sector business logic.
package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service manages tenant sectors.
type Service struct {
	repo  repository.SectorRepository
	clock shared.Clock
}

// New builds the service.
func New(repo repository.SectorRepository, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, clock: clock}
}

// Create validates and persists a sector.
func (s *Service) Create(ctx context.Context, cmd contracts.CreateSector) (*entity.Sector, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, apperror.Validation("sector name is required").
			WithDetails(map[string]any{"name": "is required"})
	}
	enabled := true
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	}
	now := s.clock.Now()
	sector := &entity.Sector{
		ID:          shared.NewID(),
		TenantID:    tenantID,
		Name:        name,
		Description: strings.TrimSpace(cmd.Description),
		Enabled:     enabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, sector); err != nil {
		return nil, err
	}
	return sector, nil
}

// Update applies the non-nil fields of cmd.
func (s *Service) Update(ctx context.Context, id string, cmd contracts.UpdateSector) (*entity.Sector, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	sector, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		name := strings.TrimSpace(*cmd.Name)
		if name == "" {
			return nil, apperror.Validation("sector name cannot be empty")
		}
		sector.Name = name
	}
	if cmd.Description != nil {
		sector.Description = strings.TrimSpace(*cmd.Description)
	}
	if cmd.Enabled != nil {
		sector.Enabled = *cmd.Enabled
	}
	sector.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, sector); err != nil {
		return nil, err
	}
	return sector, nil
}

// Delete removes a sector.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Get returns a sector by id.
func (s *Service) Get(ctx context.Context, id string) (*entity.Sector, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns a page of sectors.
func (s *Service) List(ctx context.Context, page shared.PageRequest) ([]*entity.Sector, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, page.Normalize())
}
