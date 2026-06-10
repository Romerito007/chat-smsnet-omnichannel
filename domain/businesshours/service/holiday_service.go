package service

import (
	"context"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// HolidayService manages tenant holidays.
type HolidayService struct {
	repo  repository.HolidayRepository
	clock shared.Clock
}

// NewHolidayService builds the service.
func NewHolidayService(repo repository.HolidayRepository, clock shared.Clock) *HolidayService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &HolidayService{repo: repo, clock: clock}
}

// Create creates a holiday.
func (s *HolidayService) Create(ctx context.Context, cmd contracts.CreateHoliday) (*entity.Holiday, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	date, err := normalizeDate(cmd.Date)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.Name) == "" {
		return nil, apperror.Validation("name is required").WithDetails(map[string]any{"name": "is required"})
	}
	scope, sectorIDs := resolveScope(cmd.SectorIDs)
	now := s.clock.Now()
	h := &entity.Holiday{
		ID:        shared.NewID(),
		TenantID:  tenantID,
		Date:      date,
		Name:      strings.TrimSpace(cmd.Name),
		Scope:     scope,
		SectorIDs: sectorIDs,
		Recurring: cmd.Recurring != nil && *cmd.Recurring,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, h); err != nil {
		return nil, err
	}
	return h, nil
}

// Update patches a holiday.
func (s *HolidayService) Update(ctx context.Context, id string, cmd contracts.UpdateHoliday) (*entity.Holiday, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	h, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Date != nil {
		date, derr := normalizeDate(*cmd.Date)
		if derr != nil {
			return nil, derr
		}
		h.Date = date
	}
	if cmd.Name != nil {
		if strings.TrimSpace(*cmd.Name) == "" {
			return nil, apperror.Validation("name cannot be empty").WithDetails(map[string]any{"name": "cannot be empty"})
		}
		h.Name = strings.TrimSpace(*cmd.Name)
	}
	if cmd.SectorIDs != nil {
		h.Scope, h.SectorIDs = resolveScope(*cmd.SectorIDs)
	}
	if cmd.Recurring != nil {
		h.Recurring = *cmd.Recurring
	}
	h.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, h); err != nil {
		return nil, err
	}
	return h, nil
}

// Delete removes a holiday.
func (s *HolidayService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Get returns a holiday.
func (s *HolidayService) Get(ctx context.Context, id string) (*entity.Holiday, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns the tenant's holidays.
func (s *HolidayService) List(ctx context.Context, page shared.PageRequest) ([]*entity.Holiday, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, page.Normalize())
}

// normalizeDate validates and canonicalizes a "YYYY-MM-DD" date.
func normalizeDate(date string) (string, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(date))
	if err != nil {
		return "", apperror.Validation("date must be YYYY-MM-DD").WithDetails(map[string]any{"date": "must be YYYY-MM-DD"})
	}
	return t.Format("2006-01-02"), nil
}

// resolveScope derives the scope from the provided sector ids: empty → all
// sectors, otherwise restricted to those sectors.
func resolveScope(sectorIDs []string) (entity.HolidayScope, []string) {
	cleaned := make([]string, 0, len(sectorIDs))
	for _, id := range sectorIDs {
		if id = strings.TrimSpace(id); id != "" {
			cleaned = append(cleaned, id)
		}
	}
	if len(cleaned) == 0 {
		return entity.ScopeAllSectors, nil
	}
	return entity.ScopeSectors, cleaned
}
