// Package service holds the sales-deal business logic: the opportunity CRUD, the
// manual stage move (the Kanban drag-and-drop), conversation linking and lost
// marking. Automation/copilot come in later blocks.
package service

import (
	"context"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/repository"
	pipelineentity "github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service manages sales deals.
type Service struct {
	repo          repository.DealRepository
	pipelines     contracts.PipelineLookup
	conversations contracts.ConversationLookup
	contacts      contracts.ContactChecker
	auditor       shared.Auditor
	clock         shared.Clock
}

// New builds the service. The conversation/contact lookups are optional (a nil
// conversation lookup disables CreateFromConversation; a nil contact checker skips
// contact validation).
func New(repo repository.DealRepository, pipelines contracts.PipelineLookup, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, pipelines: pipelines, auditor: shared.NoopAuditor{}, clock: clock}
}

// SetAuditor wires the audit trail (optional).
func (s *Service) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// SetConversationLookup wires the conversation resolver (CreateFromConversation).
func (s *Service) SetConversationLookup(c contracts.ConversationLookup) {
	if c != nil {
		s.conversations = c
	}
}

// SetContactChecker wires the contact-existence guard.
func (s *Service) SetContactChecker(c contracts.ContactChecker) {
	if c != nil {
		s.contacts = c
	}
}

// Create stores a new opportunity. Pipeline/stage default to the tenant's default
// pipeline and its first stage. The status follows the target stage (won/lost when
// terminal).
func (s *Service) Create(ctx context.Context, cmd contracts.CreateDeal) (*entity.Deal, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.Title) == "" {
		return nil, apperror.Validation("title is required")
	}
	if cmd.Value < 0 {
		return nil, apperror.Validation("value must be >= 0")
	}
	pl, stage, err := s.resolvePipelineStage(ctx, cmd.PipelineID, cmd.StageID)
	if err != nil {
		return nil, err
	}
	if err := s.requireContact(ctx, cmd.ContactID); err != nil {
		return nil, err
	}

	now := s.clock.Now()
	d := &entity.Deal{
		ID:                shared.NewID(),
		TenantID:          tenantID,
		PipelineID:        pl.ID,
		Title:             strings.TrimSpace(cmd.Title),
		Value:             cmd.Value,
		Currency:          currencyOr(cmd.Currency),
		ContactID:         strings.TrimSpace(cmd.ContactID),
		AssignedTo:        strings.TrimSpace(cmd.AssignedTo),
		SectorID:          strings.TrimSpace(cmd.SectorID),
		Source:            strings.TrimSpace(cmd.Source),
		ExpectedCloseDate: cmd.ExpectedCloseDate,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	applyStage(d, stage, now)
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	s.audit(ctx, "deal.created", d.ID)
	return d, nil
}

// CreateFromConversation creates a deal pre-linked to a conversation and its contact,
// placed in the tenant's default pipeline / first stage. Used by the "create
// opportunity" button inside a conversation.
func (s *Service) CreateFromConversation(ctx context.Context, cmd contracts.CreateFromConversation) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	if s.conversations == nil {
		return nil, apperror.Integration("conversation lookup is not configured")
	}
	convID := strings.TrimSpace(cmd.ConversationID)
	if convID == "" {
		return nil, apperror.Validation("conversation_id is required")
	}
	conv, err := s.conversations.Conversation(ctx, convID)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(cmd.Title)
	if title == "" {
		title = "Oportunidade"
	}
	d, err := s.Create(ctx, contracts.CreateDeal{
		Title:      title,
		Value:      cmd.Value,
		Currency:   cmd.Currency,
		ContactID:  conv.ContactID,
		SectorID:   conv.SectorID,
		AssignedTo: conv.AssignedTo,
	})
	if err != nil {
		return nil, err
	}
	return s.LinkConversation(ctx, d.ID, convID)
}

// Get returns a deal by id (tenant-scoped).
func (s *Service) Get(ctx context.Context, id string) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns a page of deals matching the filter, constrained by the actor's
// visibility (assigned-to-me or my sectors when not all-scope).
func (s *Service) List(ctx context.Context, f contracts.ListFilter, page shared.PageRequest) ([]*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	vis, err := s.visibility(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, f, vis, page.Normalize())
}

// Update edits the editable fields (title/value/currency/seller/sector/source/date).
func (s *Service) Update(ctx context.Context, id string, cmd contracts.UpdateDeal) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	d, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Title != nil {
		if strings.TrimSpace(*cmd.Title) == "" {
			return nil, apperror.Validation("title is required")
		}
		d.Title = strings.TrimSpace(*cmd.Title)
	}
	if cmd.Value != nil {
		if *cmd.Value < 0 {
			return nil, apperror.Validation("value must be >= 0")
		}
		d.Value = *cmd.Value
	}
	if cmd.Currency != nil {
		d.Currency = currencyOr(*cmd.Currency)
	}
	if cmd.AssignedTo != nil {
		d.AssignedTo = strings.TrimSpace(*cmd.AssignedTo)
	}
	if cmd.SectorID != nil {
		d.SectorID = strings.TrimSpace(*cmd.SectorID)
	}
	if cmd.Source != nil {
		d.Source = strings.TrimSpace(*cmd.Source)
	}
	if cmd.ClearExpectedDate {
		d.ExpectedCloseDate = nil
	} else if cmd.ExpectedCloseDate != nil {
		d.ExpectedCloseDate = cmd.ExpectedCloseDate
	}
	d.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	s.audit(ctx, "deal.updated", d.ID)
	return d, nil
}

// MoveStage moves a deal to another stage of its pipeline (the Kanban drag-and-drop).
// It bumps StageChangedAt, and sets won/lost+ClosedAt when the target is terminal or
// reopens (status=open, ClosedAt cleared) when it is not.
func (s *Service) MoveStage(ctx context.Context, dealID, stageID string) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	d, err := s.repo.FindByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	pl, err := s.pipelines.Get(ctx, d.PipelineID)
	if err != nil {
		return nil, err
	}
	stage := findStage(pl, stageID)
	if stage == nil {
		return nil, apperror.Validation("the stage does not belong to the deal's pipeline")
	}
	applyStage(d, stage, s.clock.Now())
	d.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	s.audit(ctx, "deal.stage_moved", d.ID)
	return d, nil
}

// AssignTo sets (or clears with "") the deal's seller.
func (s *Service) AssignTo(ctx context.Context, dealID, userID string) (*entity.Deal, error) {
	return s.Update(ctx, dealID, contracts.UpdateDeal{AssignedTo: ptr(strings.TrimSpace(userID))})
}

// LinkConversation links a conversation to the deal (idempotent).
func (s *Service) LinkConversation(ctx context.Context, dealID, conversationID string) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, apperror.Validation("conversation_id is required")
	}
	d, err := s.repo.FindByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	if d.HasConversation(conversationID) {
		return d, nil // idempotent
	}
	d.ConversationIDs = append(d.ConversationIDs, conversationID)
	d.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	s.audit(ctx, "deal.conversation_linked", d.ID)
	return d, nil
}

// MarkLost marks the deal lost: it moves it to the pipeline's lost stage when one
// exists, and records the reason. Always sets status=lost + ClosedAt.
func (s *Service) MarkLost(ctx context.Context, dealID, reason string) (*entity.Deal, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	d, err := s.repo.FindByID(ctx, dealID)
	if err != nil {
		return nil, err
	}
	pl, err := s.pipelines.Get(ctx, d.PipelineID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	if lost := lostStage(pl); lost != nil {
		applyStage(d, lost, now)
	} else {
		d.Status = entity.StatusLost
		d.ClosedAt = &now
	}
	d.LostReason = strings.TrimSpace(reason)
	d.UpdatedAt = now
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, err
	}
	s.audit(ctx, "deal.marked_lost", d.ID)
	return d, nil
}

// Delete removes a deal.
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
	s.audit(ctx, "deal.deleted", id)
	return nil
}

// StageHasDeals implements the pipelines domain's StageDealChecker, so deleting a
// stage that still holds deals is refused.
func (s *Service) StageHasDeals(ctx context.Context, pipelineID, stageID string) (bool, error) {
	n, err := s.repo.CountByStage(ctx, pipelineID, stageID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// resolvePipelineStage resolves the target pipeline (explicit or default) and stage
// (explicit or first), validating the stage belongs to the pipeline.
func (s *Service) resolvePipelineStage(ctx context.Context, pipelineID, stageID string) (*pipelineentity.Pipeline, *pipelineentity.Stage, error) {
	var (
		pl  *pipelineentity.Pipeline
		err error
	)
	if strings.TrimSpace(pipelineID) != "" {
		pl, err = s.pipelines.Get(ctx, pipelineID)
	} else {
		pl, err = s.pipelines.Default(ctx)
	}
	if err != nil {
		return nil, nil, err
	}
	if len(pl.Stages) == 0 {
		return nil, nil, apperror.Validation("the pipeline has no stages")
	}
	pl.SortStages()
	if strings.TrimSpace(stageID) == "" {
		return pl, &pl.Stages[0], nil
	}
	stage := findStage(pl, stageID)
	if stage == nil {
		return nil, nil, apperror.Validation("the stage does not belong to the pipeline")
	}
	return pl, stage, nil
}

func (s *Service) requireContact(ctx context.Context, contactID string) error {
	contactID = strings.TrimSpace(contactID)
	if contactID == "" || s.contacts == nil {
		return nil
	}
	ok, err := s.contacts.ContactExists(ctx, contactID)
	if err != nil {
		return err
	}
	if !ok {
		return apperror.Validation("contact not found")
	}
	return nil
}

func (s *Service) visibility(ctx context.Context) (contracts.Visibility, error) {
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return contracts.Visibility{}, apperror.Unauthorized("authentication required")
	}
	return contracts.Visibility{All: ac.SectorScope == authz.ScopeAll, SectorIDs: ac.SectorIDs, UserID: ac.UserID}, nil
}

func (s *Service) audit(ctx context.Context, action, id string) {
	_ = s.auditor.Record(ctx, shared.AuditEntry{Action: action, ResourceType: "deal", ResourceID: id})
}

// applyStage moves a deal into a stage and reconciles its status + closed_at:
// won/lost (with ClosedAt) when the stage is terminal, otherwise open (ClosedAt
// cleared — a reopen). StageChangedAt is always bumped.
func applyStage(d *entity.Deal, st *pipelineentity.Stage, now time.Time) {
	d.StageID = st.ID
	d.StageChangedAt = now
	switch {
	case st.IsWon:
		d.Status = entity.StatusWon
		closed := now
		d.ClosedAt = &closed
	case st.IsLost:
		d.Status = entity.StatusLost
		closed := now
		d.ClosedAt = &closed
	default:
		d.Status = entity.StatusOpen
		d.ClosedAt = nil
	}
}

func findStage(pl *pipelineentity.Pipeline, stageID string) *pipelineentity.Stage {
	i := pl.StageIndex(stageID)
	if i < 0 {
		return nil
	}
	return &pl.Stages[i]
}

func lostStage(pl *pipelineentity.Pipeline) *pipelineentity.Stage {
	for i := range pl.Stages {
		if pl.Stages[i].IsLost {
			return &pl.Stages[i]
		}
	}
	return nil
}

func currencyOr(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return entity.DefaultCurrency
	}
	return c
}

func ptr[T any](v T) *T { return &v }
