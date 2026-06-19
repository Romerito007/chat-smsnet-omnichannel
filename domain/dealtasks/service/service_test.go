package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fakeRepo struct{ byID map[string]*entity.DealTask }

func newRepo() *fakeRepo { return &fakeRepo{byID: map[string]*entity.DealTask{}} }

func (r *fakeRepo) Create(ctx context.Context, t *entity.DealTask) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	cp := *t
	r.byID[t.ID] = &cp
	return nil
}
func (r *fakeRepo) Update(ctx context.Context, t *entity.DealTask) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if _, ok := r.byID[t.ID]; !ok {
		return apperror.NotFound("nf")
	}
	cp := *t
	r.byID[t.ID] = &cp
	return nil
}
func (r *fakeRepo) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if _, ok := r.byID[id]; !ok {
		return apperror.NotFound("nf")
	}
	delete(r.byID, id)
	return nil
}
func (r *fakeRepo) FindByID(ctx context.Context, id string) (*entity.DealTask, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	if t, ok := r.byID[id]; ok {
		cp := *t
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeRepo) ListByDeal(ctx context.Context, dealID string, _ shared.PageRequest) ([]*entity.DealTask, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var out []*entity.DealTask
	for _, t := range r.byID {
		if t.TenantID == tenantID && t.DealID == dealID {
			cp := *t
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (r *fakeRepo) List(ctx context.Context, f contracts.ListFilter, _ shared.PageRequest) ([]*entity.DealTask, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var out []*entity.DealTask
	for _, t := range r.byID {
		if t.TenantID != tenantID {
			continue
		}
		if f.AssignedTo != "" && t.AssignedTo != f.AssignedTo {
			continue
		}
		if f.Status != "" && string(t.Status) != f.Status {
			continue
		}
		cp := *t
		out = append(out, &cp)
	}
	return out, nil
}

type fakeDeals struct{ ref *contracts.DealRef }

func (f fakeDeals) Deal(context.Context, string) (*contracts.DealRef, error) {
	if f.ref == nil {
		return nil, apperror.NotFound("nf")
	}
	cp := *f.ref
	return &cp, nil
}

type fakeGate struct{ on bool }

func (g fakeGate) TasksEnabled(context.Context) (bool, error) { return g.on, nil }

type fakeChecker struct{ agents map[string]bool }

func (f fakeChecker) AgentExists(_ context.Context, id string) (bool, error) {
	return f.agents[id], nil
}

type fakeCards struct{ cards map[string]shared.DisplayCard }

func (f fakeCards) AgentCards(_ context.Context, ids []string) (map[string]shared.DisplayCard, error) {
	out := map[string]shared.DisplayCard{}
	for _, id := range ids {
		if c, ok := f.cards[id]; ok {
			out[id] = c
		}
	}
	return out, nil
}

type fakeTimeline struct{ events []contracts.TimelineEvent }

func (f *fakeTimeline) Record(_ context.Context, ev contracts.TimelineEvent) {
	f.events = append(f.events, ev)
}
func (f *fakeTimeline) has(kind string) bool {
	for _, e := range f.events {
		if e.Kind == kind {
			return true
		}
	}
	return false
}
func (f *fakeTimeline) count(kind string) int {
	n := 0
	for _, e := range f.events {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

type fakeNotifier struct{ inputs []shared.NotifyInput }

func (n *fakeNotifier) Notify(_ context.Context, in shared.NotifyInput) {
	n.inputs = append(n.inputs, in)
}

func tenantCtx() context.Context { return shared.WithTenant(context.Background(), "t1") }

func authCtx(scope authz.SectorScope, sectors []string, user string) context.Context {
	return authz.WithAuthContext(tenantCtx(), authz.NewAuthContext("t1", user, authz.AllPermissions(), sectors, scope))
}

func ownerRef() *contracts.DealRef {
	return &contracts.DealRef{TenantID: "t1", SectorID: "sec1", AssignedTo: "u1"}
}

func newSvc(on bool, ref *contracts.DealRef) (*Service, *fakeRepo, *fakeTimeline, *fakeNotifier) {
	repo := newRepo()
	tl := &fakeTimeline{}
	not := &fakeNotifier{}
	svc := New(repo, fakeDeals{ref: ref}, nil)
	svc.SetModuleGate(fakeGate{on: on})
	svc.SetAgentChecker(fakeChecker{agents: map[string]bool{"u1": true, "u2": true}})
	svc.SetDirectory(fakeCards{cards: map[string]shared.DisplayCard{
		"u1": {Name: "Ana", AvatarURL: "https://cdn/ana.png"},
		"u2": {Name: "Bruno"},
	}})
	svc.SetTimeline(tl)
	svc.SetNotifier(not)
	return svc, repo, tl, not
}

func TestCreate_PersistsEmitsTimelineAndNotifies(t *testing.T) {
	svc, repo, tl, not := newSvc(true, ownerRef())
	ctx := authCtx(authz.ScopeAll, nil, "u1") // creator u1

	v, err := svc.Create(ctx, contracts.CreateTask{DealID: "d1", Title: "Ligar para o cliente", AssignedTo: "u2"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if v.Status != "pending" || v.CreatedBy != "u1" || v.AssignedTo != "u2" {
		t.Errorf("task fields wrong: %+v", v)
	}
	// Enriched names.
	if v.AssignedToName != "Bruno" || v.CreatedByName != "Ana" {
		t.Errorf("names not enriched: %+v", v)
	}
	if len(repo.byID) != 1 {
		t.Errorf("task not persisted")
	}
	if !tl.has("task_created") {
		t.Errorf("task_created not recorded on the timeline")
	}
	// Assignee (u2 ≠ creator u1) notified.
	if len(not.inputs) != 1 || not.inputs[0].UserID != "u2" || not.inputs[0].Type != "deal.task_assigned" {
		t.Errorf("assignee not notified: %+v", not.inputs)
	}
}

func TestCreate_ValidatesTitleAndAssignee(t *testing.T) {
	svc, _, _, _ := newSvc(true, ownerRef())
	ctx := authCtx(authz.ScopeAll, nil, "u1")

	if _, err := svc.Create(ctx, contracts.CreateTask{DealID: "d1", Title: "  "}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("empty title must be validation, got %v", err)
	}
	if _, err := svc.Create(ctx, contracts.CreateTask{DealID: "d1", Title: "x", AssignedTo: "ghost"}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("unknown assignee must be validation, got %v", err)
	}
}

func TestCreate_RejectedWhenModuleDisabled(t *testing.T) {
	svc, _, _, _ := newSvc(false, ownerRef())
	ctx := authCtx(authz.ScopeAll, nil, "u1")
	if _, err := svc.Create(ctx, contracts.CreateTask{DealID: "d1", Title: "x"}); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("disabled module must be a conflict, got %v", err)
	}
}

func TestComplete_MarksDoneEmitsAndIsIdempotent(t *testing.T) {
	svc, _, tl, _ := newSvc(true, ownerRef())
	ctx := authCtx(authz.ScopeAll, nil, "u1")
	v, _ := svc.Create(ctx, contracts.CreateTask{DealID: "d1", Title: "x"})

	done, err := svc.Complete(ctx, "d1", v.ID)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if done.Status != "done" || done.CompletedAt == nil {
		t.Errorf("task not completed: %+v", done)
	}
	if tl.count("task_completed") != 1 {
		t.Errorf("task_completed must be recorded once, got %d", tl.count("task_completed"))
	}
	// Completing again is idempotent — no second event.
	if _, err := svc.Complete(ctx, "d1", v.ID); err != nil {
		t.Fatalf("complete again: %v", err)
	}
	if tl.count("task_completed") != 1 {
		t.Errorf("re-completing must not emit another event, got %d", tl.count("task_completed"))
	}
}

func TestUpdate_ReassignNotifies(t *testing.T) {
	svc, _, _, not := newSvc(true, ownerRef())
	ctx := authCtx(authz.ScopeAll, nil, "u1")
	v, _ := svc.Create(ctx, contracts.CreateTask{DealID: "d1", Title: "x"}) // unassigned, no notify
	if len(not.inputs) != 0 {
		t.Fatalf("unassigned create must not notify")
	}
	if _, err := svc.Update(ctx, "d1", v.ID, contracts.UpdateTask{AssignedTo: ptrStr("u2")}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(not.inputs) != 1 || not.inputs[0].UserID != "u2" {
		t.Errorf("reassign must notify the new assignee: %+v", not.inputs)
	}
}

func TestListByDeal_EmptyWhenDisabledAndRespectsVisibility(t *testing.T) {
	// Disabled → empty.
	off, _, _, _ := newSvc(false, ownerRef())
	if items, err := off.ListByDeal(authCtx(authz.ScopeAll, nil, "x"), "d1", shared.PageRequest{Limit: 10}); err != nil || len(items) != 0 {
		t.Errorf("disabled must return empty, got %v %v", items, err)
	}

	// Enabled but the actor can't see the deal → NotFound.
	svc, _, _, _ := newSvc(true, ownerRef()) // deal in sec1, assigned u1
	other := authCtx(authz.ScopeOwn, []string{"sec9"}, "u9")
	if _, err := svc.ListByDeal(other, "d1", shared.PageRequest{Limit: 10}); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("a non-visible deal's tasks must be NotFound, got %v", err)
	}
	// A sector member sees it.
	member := authCtx(authz.ScopeOwn, []string{"sec1"}, "u9")
	if _, err := svc.ListByDeal(member, "d1", shared.PageRequest{Limit: 10}); err != nil {
		t.Errorf("a sector member must see the tasks: %v", err)
	}
}

func TestListMine_NonAllScopeSeesOnlyOwn(t *testing.T) {
	svc, repo, _, _ := newSvc(true, ownerRef())
	now := time.Now()
	repo.byID["a"] = &entity.DealTask{ID: "a", TenantID: "t1", DealID: "d1", Title: "mine", AssignedTo: "u9", Status: entity.StatusPending, CreatedAt: now}
	repo.byID["b"] = &entity.DealTask{ID: "b", TenantID: "t1", DealID: "d2", Title: "theirs", AssignedTo: "u2", Status: entity.StatusPending, CreatedAt: now}

	// Non-all-scope u9 → only its own task, regardless of the assigned_to filter.
	mine, err := svc.ListMine(authCtx(authz.ScopeOwn, []string{"sec1"}, "u9"), contracts.ListFilter{AssignedTo: "u2"}, shared.PageRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list mine: %v", err)
	}
	if len(mine) != 1 || mine[0].ID != "a" {
		t.Errorf("non-all-scope must see only own tasks, got %+v", mine)
	}

	// All-scope can filter freely.
	all, _ := svc.ListMine(authCtx(authz.ScopeAll, nil, "boss"), contracts.ListFilter{AssignedTo: "u2"}, shared.PageRequest{Limit: 10})
	if len(all) != 1 || all[0].ID != "b" {
		t.Errorf("all-scope must filter freely, got %+v", all)
	}
}

func ptrStr(s string) *string { return &s }
