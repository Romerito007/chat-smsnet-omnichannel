// Package service holds the queue business logic.
package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/queues/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/queues/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/queues/repository"
	sectorrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service manages tenant queues. It validates that the referenced sector exists
// within the same tenant.
type Service struct {
	repo    repository.QueueRepository
	sectors sectorrepo.SectorRepository
	clock   shared.Clock
}

// New builds the service.
func New(repo repository.QueueRepository, sectors sectorrepo.SectorRepository, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, sectors: sectors, clock: clock}
}

// Create validates and persists a queue.
func (s *Service) Create(ctx context.Context, cmd contracts.CreateQueue) (*entity.Queue, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}

	v := map[string]any{}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		v["name"] = "is required"
	}
	if strings.TrimSpace(cmd.SectorID) == "" {
		v["sector_id"] = "is required"
	}
	strategy := cmd.Strategy
	if strategy == "" {
		strategy = entity.StrategyManual
	}
	if !strategy.Valid() {
		v["strategy"] = "must be one of manual|round_robin|least_loaded|priority"
	}
	if cmd.MaxWaitSeconds < 0 {
		v["max_wait_seconds"] = "cannot be negative"
	}
	if len(v) > 0 {
		return nil, apperror.Validation("validation failed").WithDetails(v)
	}

	// The sector must exist within the tenant (FindByID is tenant-scoped).
	if _, err := s.sectors.FindByID(ctx, cmd.SectorID); err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return nil, apperror.Validation("sector does not exist").
				WithDetails(map[string]any{"sector_id": "not found"})
		}
		return nil, err
	}

	enabled := true
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	}
	tenantID, _ := shared.TenantFrom(ctx)
	now := s.clock.Now()
	queue := &entity.Queue{
		ID:             shared.NewID(),
		TenantID:       tenantID,
		SectorID:       cmd.SectorID,
		Name:           name,
		Strategy:       strategy,
		MaxWaitSeconds: cmd.MaxWaitSeconds,
		Enabled:        enabled,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.repo.Create(ctx, queue); err != nil {
		return nil, err
	}
	return queue, nil
}

// Update applies the non-nil fields of cmd.
func (s *Service) Update(ctx context.Context, id string, cmd contracts.UpdateQueue) (*entity.Queue, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	queue, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		name := strings.TrimSpace(*cmd.Name)
		if name == "" {
			return nil, apperror.Validation("queue name cannot be empty")
		}
		queue.Name = name
	}
	if cmd.Strategy != nil {
		if !cmd.Strategy.Valid() {
			return nil, apperror.Validation("invalid strategy")
		}
		queue.Strategy = *cmd.Strategy
	}
	if cmd.MaxWaitSeconds != nil {
		if *cmd.MaxWaitSeconds < 0 {
			return nil, apperror.Validation("max_wait_seconds cannot be negative")
		}
		queue.MaxWaitSeconds = *cmd.MaxWaitSeconds
	}
	if cmd.Enabled != nil {
		queue.Enabled = *cmd.Enabled
	}
	queue.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, queue); err != nil {
		return nil, err
	}
	return queue, nil
}

// Delete removes a queue.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Get returns a queue by id.
func (s *Service) Get(ctx context.Context, id string) (*entity.Queue, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns a page of queues.
func (s *Service) List(ctx context.Context, page shared.PageRequest) ([]*entity.Queue, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, page.Normalize())
}
