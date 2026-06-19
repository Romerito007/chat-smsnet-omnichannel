// Package service holds the deal-task business logic: CRUD of seller follow-ups on a
// deal, gated by the tenant's tasks toggle, constrained by the deal's visibility,
// recording task_created/task_completed on the deal's timeline and (best-effort)
// notifying the assignee. Names are resolved in batch (never a raw id).
package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service manages deal tasks.
type Service struct {
	repo     repository.TaskRepository
	deals    contracts.DealLookup
	gate     contracts.ModuleGate
	agents   contracts.AgentChecker
	cards    contracts.AgentDirectory
	timeline contracts.TimelineWriter
	notifier shared.Notifier
	clock    shared.Clock
}

// New builds the service. The deal lookup is required for the visibility checks.
func New(repo repository.TaskRepository, deals contracts.DealLookup, clock shared.Clock) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: repo, deals: deals, notifier: shared.NoopNotifier{}, clock: clock}
}

// SetModuleGate wires the tasks on/off toggle (crmsettings). Optional.
func (s *Service) SetModuleGate(g contracts.ModuleGate) {
	if g != nil {
		s.gate = g
	}
}

// SetAgentChecker wires the assignee validator (agent of the tenant). Optional.
func (s *Service) SetAgentChecker(a contracts.AgentChecker) {
	if a != nil {
		s.agents = a
	}
}

// SetDirectory wires the agent name+avatar resolver for the assignee/creator. Optional.
func (s *Service) SetDirectory(d contracts.AgentDirectory) {
	if d != nil {
		s.cards = d
	}
}

// SetTimeline wires the timeline writer (task_created/task_completed). Optional.
func (s *Service) SetTimeline(w contracts.TimelineWriter) {
	if w != nil {
		s.timeline = w
	}
}

// SetNotifier wires the in-app notifier (alert the assignee). Optional.
func (s *Service) SetNotifier(n shared.Notifier) {
	if n != nil {
		s.notifier = n
	}
}

// Create stores a new task on a deal and records task_created on its timeline.
func (s *Service) Create(ctx context.Context, cmd contracts.CreateTask) (*contracts.TaskView, error) {
	tenantID, err := s.guardWrite(ctx)
	if err != nil {
		return nil, err
	}
	dealID := strings.TrimSpace(cmd.DealID)
	if _, err := s.ensureVisible(ctx, dealID); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(cmd.Title)
	if title == "" {
		return nil, apperror.Validation("title is required")
	}
	assignee := strings.TrimSpace(cmd.AssignedTo)
	if err := s.validateAssignee(ctx, assignee); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	t := &entity.DealTask{
		ID: shared.NewID(), TenantID: tenantID, DealID: dealID, Title: title,
		Description: strings.TrimSpace(cmd.Description), DueDate: cmd.DueDate,
		AssignedTo: assignee, Status: entity.StatusPending,
		CreatedBy: actorFromCtx(ctx), CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	s.record(ctx, t, "task_created")
	s.notifyAssignee(ctx, t)
	return s.view(ctx, t), nil
}

// Update edits a task's fields (not its status — use Complete).
func (s *Service) Update(ctx context.Context, dealID, taskID string, cmd contracts.UpdateTask) (*contracts.TaskView, error) {
	if _, err := s.guardWrite(ctx); err != nil {
		return nil, err
	}
	t, err := s.load(ctx, dealID, taskID)
	if err != nil {
		return nil, err
	}
	prevAssignee := t.AssignedTo
	if cmd.Title != nil {
		if strings.TrimSpace(*cmd.Title) == "" {
			return nil, apperror.Validation("title is required")
		}
		t.Title = strings.TrimSpace(*cmd.Title)
	}
	if cmd.Description != nil {
		t.Description = strings.TrimSpace(*cmd.Description)
	}
	if cmd.ClearDueDate {
		t.DueDate = nil
	} else if cmd.DueDate != nil {
		t.DueDate = cmd.DueDate
	}
	if cmd.AssignedTo != nil {
		assignee := strings.TrimSpace(*cmd.AssignedTo)
		if err := s.validateAssignee(ctx, assignee); err != nil {
			return nil, err
		}
		t.AssignedTo = assignee
	}
	t.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	if t.AssignedTo != "" && t.AssignedTo != prevAssignee {
		s.notifyAssignee(ctx, t)
	}
	return s.view(ctx, t), nil
}

// Complete marks a task done (idempotent) and records task_completed on the timeline.
func (s *Service) Complete(ctx context.Context, dealID, taskID string) (*contracts.TaskView, error) {
	if _, err := s.guardWrite(ctx); err != nil {
		return nil, err
	}
	t, err := s.load(ctx, dealID, taskID)
	if err != nil {
		return nil, err
	}
	if t.IsDone() {
		return s.view(ctx, t), nil // idempotent: no duplicate event
	}
	now := s.clock.Now()
	t.Status = entity.StatusDone
	t.CompletedAt = &now
	t.UpdatedAt = now
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	s.record(ctx, t, "task_completed")
	return s.view(ctx, t), nil
}

// Delete removes a task.
func (s *Service) Delete(ctx context.Context, dealID, taskID string) error {
	if _, err := s.guardWrite(ctx); err != nil {
		return err
	}
	t, err := s.load(ctx, dealID, taskID)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, t.ID)
}

// ListByDeal returns a deal's tasks (empty when the module is off).
func (s *Service) ListByDeal(ctx context.Context, dealID string, page shared.PageRequest) ([]contracts.TaskView, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	on, err := s.enabled(ctx)
	if err != nil {
		return nil, err
	}
	if !on {
		return []contracts.TaskView{}, nil
	}
	if _, err := s.ensureVisible(ctx, dealID); err != nil {
		return nil, err
	}
	tasks, err := s.repo.ListByDeal(ctx, dealID, page.Normalize())
	if err != nil {
		return nil, err
	}
	return s.views(ctx, tasks), nil
}

// ListMine returns the consolidated task view across deals (the "my tasks" board),
// empty when the module is off. A non-all-scope actor only sees its OWN tasks (the
// assigned_to filter is forced to the actor); an all-scope actor filters freely.
func (s *Service) ListMine(ctx context.Context, f contracts.ListFilter, page shared.PageRequest) ([]contracts.TaskView, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	on, err := s.enabled(ctx)
	if err != nil {
		return nil, err
	}
	if !on {
		return []contracts.TaskView{}, nil
	}
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return nil, apperror.Unauthorized("authentication required")
	}
	if ac.SectorScope != authz.ScopeAll {
		f.AssignedTo = ac.UserID // own tasks only
	}
	tasks, err := s.repo.List(ctx, f, page.Normalize())
	if err != nil {
		return nil, err
	}
	return s.views(ctx, tasks), nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// guardWrite enforces the tenant and that the tasks module is enabled (409 when off).
func (s *Service) guardWrite(ctx context.Context) (string, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return "", err
	}
	on, err := s.enabled(ctx)
	if err != nil {
		return "", err
	}
	if !on {
		return "", apperror.Conflict("the tasks module is disabled for this tenant")
	}
	return tenantID, nil
}

func (s *Service) enabled(ctx context.Context) (bool, error) {
	if s.gate == nil {
		return true, nil
	}
	return s.gate.TasksEnabled(ctx)
}

// load fetches a task and verifies it belongs to the (visible) deal.
func (s *Service) load(ctx context.Context, dealID, taskID string) (*entity.DealTask, error) {
	if _, err := s.ensureVisible(ctx, dealID); err != nil {
		return nil, err
	}
	t, err := s.repo.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if t.DealID != dealID {
		return nil, apperror.NotFound("task not found")
	}
	return t, nil
}

// ensureVisible loads the deal and enforces the deal visibility (all-scope, assignee
// or sector member); a non-visible deal is reported as NotFound.
func (s *Service) ensureVisible(ctx context.Context, dealID string) (*contracts.DealRef, error) {
	if strings.TrimSpace(dealID) == "" {
		return nil, apperror.Validation("deal id is required")
	}
	ref, err := s.deals.Deal(ctx, dealID)
	if err != nil {
		return nil, err
	}
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return nil, apperror.Unauthorized("authentication required")
	}
	if ac.SectorScope == authz.ScopeAll || ref.AssignedTo == ac.UserID {
		return ref, nil
	}
	for _, sid := range ac.SectorIDs {
		if sid != "" && sid == ref.SectorID {
			return ref, nil
		}
	}
	return nil, apperror.NotFound("deal not found")
}

// validateAssignee checks that a non-empty assignee is an agent of the tenant.
func (s *Service) validateAssignee(ctx context.Context, userID string) error {
	if userID == "" || s.agents == nil {
		return nil
	}
	ok, err := s.agents.AgentExists(ctx, userID)
	if err != nil {
		return err
	}
	if !ok {
		return apperror.Validation("assigned_to is not an agent of the tenant")
	}
	return nil
}

// record writes a task event to the deal's timeline (best-effort).
func (s *Service) record(ctx context.Context, t *entity.DealTask, kind string) {
	if s.timeline == nil {
		return
	}
	s.timeline.Record(ctx, contracts.TimelineEvent{
		DealID: t.DealID, Kind: kind, ActorID: actorFromCtx(ctx),
		Data: map[string]any{"task_id": t.ID, "title": t.Title},
	})
}

// notifyAssignee alerts the assignee (when set and not the actor) in-app.
func (s *Service) notifyAssignee(ctx context.Context, t *entity.DealTask) {
	if t.AssignedTo == "" || t.AssignedTo == actorFromCtx(ctx) {
		return
	}
	s.notifier.Notify(ctx, shared.NotifyInput{
		TenantID: t.TenantID,
		UserID:   t.AssignedTo,
		Type:     "deal.task_assigned",
		Title:    "Uma tarefa do CRM foi atribuída a você",
		Body:     t.Title,
		Link:     "/crm?deal=" + t.DealID + "&task=" + t.ID,
	})
}

func (s *Service) views(ctx context.Context, tasks []*entity.DealTask) []contracts.TaskView {
	out := make([]contracts.TaskView, 0, len(tasks))
	cards := s.cardsFor(ctx, tasks)
	for _, t := range tasks {
		out = append(out, enrich(t, cards))
	}
	return out
}

func (s *Service) view(ctx context.Context, t *entity.DealTask) *contracts.TaskView {
	cards := s.cardsFor(ctx, []*entity.DealTask{t})
	v := enrich(t, cards)
	return &v
}

// cardsFor batches every assignee + creator id of the tasks into ONE directory call.
func (s *Service) cardsFor(ctx context.Context, tasks []*entity.DealTask) map[string]shared.DisplayCard {
	if s.cards == nil || len(tasks) == 0 {
		return map[string]shared.DisplayCard{}
	}
	set := map[string]struct{}{}
	for _, t := range tasks {
		if t.AssignedTo != "" {
			set[t.AssignedTo] = struct{}{}
		}
		if t.CreatedBy != "" {
			set[t.CreatedBy] = struct{}{}
		}
	}
	if len(set) == 0 {
		return map[string]shared.DisplayCard{}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	cards, err := s.cards.AgentCards(ctx, ids)
	if err != nil {
		return map[string]shared.DisplayCard{}
	}
	return cards
}

func enrich(t *entity.DealTask, cards map[string]shared.DisplayCard) contracts.TaskView {
	v := contracts.TaskView{
		ID: t.ID, DealID: t.DealID, Title: t.Title, Description: t.Description,
		DueDate: t.DueDate, AssignedTo: t.AssignedTo, Status: string(t.Status),
		CompletedAt: t.CompletedAt, CreatedBy: t.CreatedBy, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
	if c, ok := cards[t.AssignedTo]; ok {
		v.AssignedToName = c.Name
		v.AssignedToAvatarURL = c.AvatarURL
	}
	if c, ok := cards[t.CreatedBy]; ok {
		v.CreatedByName = c.Name
	}
	return v
}

func actorFromCtx(ctx context.Context) string {
	if ac, ok := authz.FromContext(ctx); ok && ac.UserID != "" {
		return ac.UserID
	}
	return "system"
}
