// Package service holds the sales-pipeline business logic: the tenant-configurable
// funnel and its stages. Deals (opportunities) are a later block; this service only
// manages the funnel shape.
package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service manages sales pipelines and their stages.
type Service struct {
	repo    repository.PipelineRepository
	deals   contracts.StageDealChecker
	auditor shared.Auditor
	clock   shared.Clock
}

// New builds the service.
func New(repo repository.PipelineRepository, clock shared.Clock) *Service {
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

// SetDealChecker wires the guard that prevents deleting a stage that still holds
// deals. Optional until the deals block exists.
func (s *Service) SetDealChecker(c contracts.StageDealChecker) {
	if c != nil {
		s.deals = c
	}
}

// Create stores a new pipeline. The tenant's FIRST pipeline becomes the default
// (used by the Kanban when none is selected). Stages get ids and are validated.
func (s *Service) Create(ctx context.Context, cmd contracts.CreatePipeline) (*entity.Pipeline, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	p := &entity.Pipeline{
		ID:        shared.NewID(),
		TenantID:  tenantID,
		Name:      strings.TrimSpace(cmd.Name),
		Stages:    make([]entity.Stage, 0, len(cmd.Stages)),
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, st := range cmd.Stages {
		p.Stages = append(p.Stages, entity.Stage{
			ID: shared.NewID(), Name: strings.TrimSpace(st.Name), Order: st.Order,
			IsWon: st.IsWon, IsLost: st.IsLost, Color: strings.TrimSpace(st.Color),
		})
	}
	if msg := p.Validate(); msg != "" {
		return nil, apperror.Validation(msg)
	}
	count, err := s.repo.CountByTenant(ctx)
	if err != nil {
		return nil, err
	}
	p.IsDefault = count == 0
	p.SortStages()
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	s.audit(ctx, "pipeline.created", p.ID)
	return p, nil
}

// List returns the tenant's pipelines (stages sorted within each).
func (s *Service) List(ctx context.Context) ([]*entity.Pipeline, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	ps, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		p.SortStages()
	}
	return ps, nil
}

// Get returns a pipeline by id (tenant-scoped), stages sorted.
func (s *Service) Get(ctx context.Context, id string) (*entity.Pipeline, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	p.SortStages()
	return p, nil
}

// Update renames the pipeline and/or sets it as the tenant default. Setting it
// default unsets the flag on every other pipeline (only one default per tenant).
func (s *Service) Update(ctx context.Context, id string, cmd contracts.UpdatePipeline) (*entity.Pipeline, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		p.Name = strings.TrimSpace(*cmd.Name)
	}
	if cmd.IsDefault != nil {
		p.IsDefault = *cmd.IsDefault
	}
	if msg := p.Validate(); msg != "" {
		return nil, apperror.Validation(msg)
	}
	if cmd.IsDefault != nil && *cmd.IsDefault {
		if err := s.repo.ClearDefault(ctx, p.ID); err != nil {
			return nil, err
		}
	}
	p.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	p.SortStages()
	s.audit(ctx, "pipeline.updated", p.ID)
	return p, nil
}

// Delete removes a pipeline.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.audit(ctx, "pipeline.deleted", id)
	return nil
}

// AddStage appends a stage to a pipeline.
func (s *Service) AddStage(ctx context.Context, pipelineID string, cmd contracts.AddStage) (*entity.Pipeline, error) {
	return s.mutateStages(ctx, pipelineID, "pipeline.stage_added", func(p *entity.Pipeline) error {
		p.Stages = append(p.Stages, entity.Stage{
			ID: shared.NewID(), Name: strings.TrimSpace(cmd.Name), Order: cmd.Order,
			IsWon: cmd.IsWon, IsLost: cmd.IsLost, Color: strings.TrimSpace(cmd.Color),
		})
		return nil
	})
}

// UpdateStage edits a stage in place.
func (s *Service) UpdateStage(ctx context.Context, pipelineID, stageID string, cmd contracts.UpdateStage) (*entity.Pipeline, error) {
	return s.mutateStages(ctx, pipelineID, "pipeline.stage_updated", func(p *entity.Pipeline) error {
		i := p.StageIndex(stageID)
		if i < 0 {
			return apperror.NotFound("stage not found")
		}
		st := &p.Stages[i]
		if cmd.Name != nil {
			st.Name = strings.TrimSpace(*cmd.Name)
		}
		if cmd.Order != nil {
			st.Order = *cmd.Order
		}
		if cmd.IsWon != nil {
			st.IsWon = *cmd.IsWon
		}
		if cmd.IsLost != nil {
			st.IsLost = *cmd.IsLost
		}
		if cmd.Color != nil {
			st.Color = strings.TrimSpace(*cmd.Color)
		}
		return nil
	})
}

// ReorderStages assigns each listed stage its new Order from its position. Every
// existing stage id must be present exactly once.
func (s *Service) ReorderStages(ctx context.Context, pipelineID string, cmd contracts.ReorderStages) (*entity.Pipeline, error) {
	return s.mutateStages(ctx, pipelineID, "pipeline.stages_reordered", func(p *entity.Pipeline) error {
		if len(cmd.StageIDs) != len(p.Stages) {
			return apperror.Validation("reorder must list every stage exactly once")
		}
		pos := make(map[string]int, len(cmd.StageIDs))
		for i, id := range cmd.StageIDs {
			if p.StageIndex(id) < 0 {
				return apperror.Validation("unknown stage id in reorder").WithDetails(map[string]any{"stage_id": id})
			}
			if _, dup := pos[id]; dup {
				return apperror.Validation("duplicate stage id in reorder")
			}
			pos[id] = i
		}
		for i := range p.Stages {
			p.Stages[i].Order = pos[p.Stages[i].ID]
		}
		return nil
	})
}

// DeleteStage removes a stage, refusing when it still holds deals (once the deals
// block wires the checker).
func (s *Service) DeleteStage(ctx context.Context, pipelineID, stageID string) (*entity.Pipeline, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	p, err := s.repo.FindByID(ctx, pipelineID)
	if err != nil {
		return nil, err
	}
	i := p.StageIndex(stageID)
	if i < 0 {
		return nil, apperror.NotFound("stage not found")
	}
	if s.deals != nil {
		has, err := s.deals.StageHasDeals(ctx, pipelineID, stageID)
		if err != nil {
			return nil, err
		}
		if has {
			return nil, apperror.Conflict("the stage still has deals; move them before deleting")
		}
	}
	p.Stages = append(p.Stages[:i], p.Stages[i+1:]...)
	if msg := p.Validate(); msg != "" {
		return nil, apperror.Validation(msg)
	}
	p.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	p.SortStages()
	s.audit(ctx, "pipeline.stage_deleted", p.ID)
	return p, nil
}

// mutateStages loads the pipeline, applies a stage mutation, re-validates, persists
// and returns the sorted result — the shared shape of the stage operations.
func (s *Service) mutateStages(ctx context.Context, pipelineID, action string, mutate func(*entity.Pipeline) error) (*entity.Pipeline, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	p, err := s.repo.FindByID(ctx, pipelineID)
	if err != nil {
		return nil, err
	}
	if err := mutate(p); err != nil {
		return nil, err
	}
	if msg := p.Validate(); msg != "" {
		return nil, apperror.Validation(msg)
	}
	p.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	p.SortStages()
	s.audit(ctx, action, p.ID)
	return p, nil
}

func (s *Service) audit(ctx context.Context, action, id string) {
	_ = s.auditor.Record(ctx, shared.AuditEntry{Action: action, ResourceType: "pipeline", ResourceID: id})
}
