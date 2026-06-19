package service

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/entity"
	pipelineentity "github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// fakeRepo is an in-memory TimelineRepository (most-recent-first on read).
type fakeRepo struct{ events []*entity.Event }

func (r *fakeRepo) Append(ctx context.Context, ev *entity.Event) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	r.events = append(r.events, ev)
	return nil
}

func (r *fakeRepo) ListByDeal(ctx context.Context, dealID string, page shared.PageRequest) ([]*entity.Event, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var out []*entity.Event
	for _, e := range r.events {
		if e.TenantID == tenantID && e.DealID == dealID {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

type fakeDeals struct{ ref *contracts.DealRef }

func (f fakeDeals) Deal(_ context.Context, _ string) (*contracts.DealRef, error) {
	if f.ref == nil {
		return nil, apperror.NotFound("nf")
	}
	cp := *f.ref
	return &cp, nil
}

type fakeGate struct{ on bool }

func (g fakeGate) TimelineEnabled(context.Context) (bool, error) { return g.on, nil }

type fakeAgents struct{ cards map[string]shared.DisplayCard }

func (f fakeAgents) AgentCards(_ context.Context, ids []string) (map[string]shared.DisplayCard, error) {
	out := map[string]shared.DisplayCard{}
	for _, id := range ids {
		if c, ok := f.cards[id]; ok {
			out[id] = c
		}
	}
	return out, nil
}

type fakePipelines struct{ pl *pipelineentity.Pipeline }

func (f fakePipelines) Get(_ context.Context, id string) (*pipelineentity.Pipeline, error) {
	if f.pl != nil && f.pl.ID == id {
		return f.pl, nil
	}
	return nil, apperror.NotFound("nf")
}

// incClock advances one second per Now() so appended events get a stable order.
type incClock struct{ t time.Time }

func (c *incClock) Now() time.Time { c.t = c.t.Add(time.Second); return c.t }

func samplePipeline() *pipelineentity.Pipeline {
	return &pipelineentity.Pipeline{ID: "p1", TenantID: "t1", Stages: []pipelineentity.Stage{
		{ID: "s1", Name: "Novo"}, {ID: "s2", Name: "Em contato"}, {ID: "sw", Name: "Ganho", IsWon: true},
	}}
}

func tenantCtx() context.Context { return shared.WithTenant(context.Background(), "t1") }

func authCtx(scope authz.SectorScope, sectors []string, user string) context.Context {
	return authz.WithAuthContext(tenantCtx(), authz.NewAuthContext("t1", user, authz.AllPermissions(), sectors, scope))
}

func newSvc(on bool, ref *contracts.DealRef) (*Service, *fakeRepo) {
	repo := &fakeRepo{}
	svc := New(repo, fakeDeals{ref: ref}, &incClock{t: time.Unix(1700000000, 0).UTC()})
	svc.SetModuleGate(fakeGate{on: on})
	svc.SetDirectories(
		fakeAgents{cards: map[string]shared.DisplayCard{
			"u1": {Name: "Ana", AvatarURL: "https://cdn/ana.png"},
			"u2": {Name: "Bruno"},
		}},
		fakePipelines{pl: samplePipeline()},
	)
	return svc, repo
}

func ownerRef() *contracts.DealRef {
	return &contracts.DealRef{TenantID: "t1", SectorID: "sec1", AssignedTo: "u1", PipelineID: "p1"}
}

func TestRecord_AppendsEvenWhenDisabled(t *testing.T) {
	svc, repo := newSvc(false, ownerRef()) // module OFF
	svc.Record(tenantCtx(), contracts.RecordEvent{DealID: "d1", Kind: entity.KindDealCreated, ActorID: "u1", Data: map[string]any{"title": "Acme"}})
	if len(repo.events) != 1 {
		t.Fatalf("automatic writes must persist even when the module is off, got %d", len(repo.events))
	}
}

func TestRecord_DropsInvalidKind(t *testing.T) {
	svc, repo := newSvc(true, ownerRef())
	svc.Record(tenantCtx(), contracts.RecordEvent{DealID: "d1", Kind: entity.Kind("nope")})
	if len(repo.events) != 0 {
		t.Errorf("an unknown kind must be dropped")
	}
}

func TestFeed_OrderedAndEnriched(t *testing.T) {
	svc, _ := newSvc(true, ownerRef())
	ctx := authCtx(authz.ScopeAll, nil, "owner")

	svc.Record(ctx, contracts.RecordEvent{DealID: "d1", Kind: entity.KindDealCreated, ActorID: "u1", Data: map[string]any{"stage_id": "s1"}})
	svc.Record(ctx, contracts.RecordEvent{DealID: "d1", Kind: entity.KindStageChanged, ActorID: "u1", Data: map[string]any{"from_stage_id": "s1", "to_stage_id": "s2"}})
	svc.Record(ctx, contracts.RecordEvent{DealID: "d1", Kind: entity.KindAssignedChanged, ActorID: "u1", Data: map[string]any{"from": "u1", "to": "u2"}})

	feed, err := svc.Feed(ctx, "d1", shared.PageRequest{Limit: 10})
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	if len(feed) != 3 {
		t.Fatalf("expected 3 events, got %d", len(feed))
	}
	// Most recent first: assigned_changed, stage_changed, deal_created.
	if feed[0].Kind != "assigned_changed" || feed[2].Kind != "deal_created" {
		t.Errorf("feed not ordered most-recent-first: %v / %v", feed[0].Kind, feed[2].Kind)
	}
	// Actor enriched (name + avatar).
	if feed[0].ActorName != "Ana" || feed[0].ActorAvatarURL != "https://cdn/ana.png" {
		t.Errorf("actor not enriched: %+v", feed[0])
	}
	// stage_changed: stage names resolved.
	sc := feed[1]
	if sc.Data["from_stage_name"] != "Novo" || sc.Data["to_stage_name"] != "Em contato" {
		t.Errorf("stage names not resolved: %+v", sc.Data)
	}
	// assigned_changed: from/to seller names resolved.
	ac := feed[0]
	if ac.Data["from_name"] != "Ana" || ac.Data["to_name"] != "Bruno" {
		t.Errorf("assignee names not resolved: %+v", ac.Data)
	}
}

func TestFeed_EmptyWhenModuleDisabled(t *testing.T) {
	svc, _ := newSvc(false, ownerRef())
	ctx := authCtx(authz.ScopeAll, nil, "owner")
	svc.Record(ctx, contracts.RecordEvent{DealID: "d1", Kind: entity.KindComment, ActorID: "u1", Data: map[string]any{"text": "hi"}})

	feed, err := svc.Feed(ctx, "d1", shared.PageRequest{Limit: 10})
	if err != nil {
		t.Fatalf("feed: %v", err)
	}
	if len(feed) != 0 {
		t.Errorf("a disabled module must return an empty feed, got %d", len(feed))
	}
}

func TestFeed_RespectsVisibility(t *testing.T) {
	svc, _ := newSvc(true, ownerRef()) // deal in sec1, assigned u1
	// An agent in another sector, not the assignee → must not see it.
	ctx := authCtx(authz.ScopeOwn, []string{"other"}, "u9")
	if _, err := svc.Feed(ctx, "d1", shared.PageRequest{Limit: 10}); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("a non-visible deal's timeline must be NotFound, got %v", err)
	}
	// The sector's agent can see it.
	ok := authCtx(authz.ScopeOwn, []string{"sec1"}, "u9")
	if _, err := svc.Feed(ok, "d1", shared.PageRequest{Limit: 10}); err != nil {
		t.Errorf("a sector member must see the timeline: %v", err)
	}
}

func TestComment_WritesAndEnriches(t *testing.T) {
	svc, repo := newSvc(true, ownerRef())
	ctx := authCtx(authz.ScopeOwn, []string{"sec1"}, "u1")

	item, err := svc.Comment(ctx, "d1", "  Liguei para o cliente  ")
	if err != nil {
		t.Fatalf("comment: %v", err)
	}
	if item.Kind != "comment" || item.Data["text"] != "Liguei para o cliente" {
		t.Errorf("comment not recorded/trimmed: %+v", item)
	}
	if item.ActorID != "u1" || item.ActorName != "Ana" {
		t.Errorf("comment actor wrong: %+v", item)
	}
	if len(repo.events) != 1 || repo.events[0].Kind != entity.KindComment {
		t.Errorf("comment not persisted: %+v", repo.events)
	}
}

func TestComment_RejectedWhenDisabled(t *testing.T) {
	svc, _ := newSvc(false, ownerRef())
	ctx := authCtx(authz.ScopeAll, nil, "u1")
	if _, err := svc.Comment(ctx, "d1", "hi"); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("commenting on a disabled timeline must be a conflict, got %v", err)
	}
}

func TestComment_RequiresText(t *testing.T) {
	svc, _ := newSvc(true, ownerRef())
	ctx := authCtx(authz.ScopeAll, nil, "u1")
	if _, err := svc.Comment(ctx, "d1", "   "); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("empty comment must be a validation error, got %v", err)
	}
}
