// Package service holds the CSAT business logic: survey CRUD, the close trigger,
// the send/expire job handlers and the public token answer.
package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/csat/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/csat/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/csat/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// SurveyService manages CSAT surveys.
type SurveyService struct {
	repo  repository.SurveyRepository
	clock shared.Clock
}

// NewSurveyService builds the service.
func NewSurveyService(repo repository.SurveyRepository, clock shared.Clock) *SurveyService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &SurveyService{repo: repo, clock: clock}
}

// Create creates a survey.
func (s *SurveyService) Create(ctx context.Context, cmd contracts.CreateSurvey) (*entity.CSATSurvey, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.Name) == "" {
		return nil, apperror.Validation("name is required").WithDetails(map[string]any{"name": "is required"})
	}
	scale := entity.Scale(strings.TrimSpace(cmd.Scale))
	if !entity.IsValidScale(scale) {
		return nil, apperror.Validation("invalid scale").WithDetails(map[string]any{"scale": "must be csat_1_5|nps_0_10|thumbs"})
	}
	if strings.TrimSpace(cmd.QuestionText) == "" {
		return nil, apperror.Validation("question_text is required").WithDetails(map[string]any{"question_text": "is required"})
	}
	if cmd.DelaySeconds < 0 {
		return nil, apperror.Validation("delay_seconds cannot be negative")
	}
	now := s.clock.Now()
	survey := &entity.CSATSurvey{
		ID:           shared.NewID(),
		TenantID:     tenantID,
		Name:         strings.TrimSpace(cmd.Name),
		Scale:        scale,
		QuestionText: strings.TrimSpace(cmd.QuestionText),
		SendOn:       entity.SendOnConversationClosed,
		SectorIDs:    cmd.SectorIDs,
		DelaySeconds: cmd.DelaySeconds,
		Enabled:      cmd.Enabled == nil || *cmd.Enabled,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Create(ctx, survey); err != nil {
		return nil, err
	}
	return survey, nil
}

// Update patches a survey.
func (s *SurveyService) Update(ctx context.Context, id string, cmd contracts.UpdateSurvey) (*entity.CSATSurvey, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	survey, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		if strings.TrimSpace(*cmd.Name) == "" {
			return nil, apperror.Validation("name cannot be empty")
		}
		survey.Name = strings.TrimSpace(*cmd.Name)
	}
	if cmd.Scale != nil {
		scale := entity.Scale(strings.TrimSpace(*cmd.Scale))
		if !entity.IsValidScale(scale) {
			return nil, apperror.Validation("invalid scale")
		}
		survey.Scale = scale
	}
	if cmd.QuestionText != nil {
		if strings.TrimSpace(*cmd.QuestionText) == "" {
			return nil, apperror.Validation("question_text cannot be empty")
		}
		survey.QuestionText = strings.TrimSpace(*cmd.QuestionText)
	}
	if cmd.SectorIDs != nil {
		survey.SectorIDs = *cmd.SectorIDs
	}
	if cmd.DelaySeconds != nil {
		if *cmd.DelaySeconds < 0 {
			return nil, apperror.Validation("delay_seconds cannot be negative")
		}
		survey.DelaySeconds = *cmd.DelaySeconds
	}
	if cmd.Enabled != nil {
		survey.Enabled = *cmd.Enabled
	}
	survey.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, survey); err != nil {
		return nil, err
	}
	return survey, nil
}

// Delete removes a survey.
func (s *SurveyService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Get returns a survey.
func (s *SurveyService) Get(ctx context.Context, id string) (*entity.CSATSurvey, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns the tenant's surveys.
func (s *SurveyService) List(ctx context.Context, page shared.PageRequest) ([]*entity.CSATSurvey, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, page.Normalize())
}
