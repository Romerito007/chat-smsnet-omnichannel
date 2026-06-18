package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/entity"
	pipelineentity "github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fakeRepo struct {
	byID       map[string]*entity.Deal
	gotFilter  contracts.ListFilter
	gotVis     contracts.Visibility
	stageCount int
}

func newRepo() *fakeRepo { return &fakeRepo{byID: map[string]*entity.Deal{}} }

func (r *fakeRepo) Create(_ context.Context, d *entity.Deal) error {
	cp := *d
	r.byID[d.ID] = &cp
	return nil
}
func (r *fakeRepo) Update(_ context.Context, d *entity.Deal) error {
	if _, ok := r.byID[d.ID]; !ok {
		return apperror.NotFound("nf")
	}
	cp := *d
	r.byID[d.ID] = &cp
	return nil
}
func (r *fakeRepo) Delete(_ context.Context, id string) error {
	if _, ok := r.byID[id]; !ok {
		return apperror.NotFound("nf")
	}
	delete(r.byID, id)
	return nil
}
func (r *fakeRepo) FindByID(_ context.Context, id string) (*entity.Deal, error) {
	if d, ok := r.byID[id]; ok {
		cp := *d
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeRepo) List(_ context.Context, f contracts.ListFilter, vis contracts.Visibility, _ shared.PageRequest) ([]*entity.Deal, error) {
	r.gotFilter = f
	r.gotVis = vis
	out := make([]*entity.Deal, 0, len(r.byID))
	for _, d := range r.byID {
		cp := *d
		out = append(out, &cp)
	}
	return out, nil
}
func (r *fakeRepo) CountByStage(context.Context, string, string) (int, error) {
	return r.stageCount, nil
}

func samplePipeline() *pipelineentity.Pipeline {
	return &pipelineentity.Pipeline{
		ID: "p1", TenantID: "t1", IsDefault: true,
		Stages: []pipelineentity.Stage{
			{ID: "s1", Name: "Novo", Order: 0},
			{ID: "sw", Name: "Ganho", Order: 1, IsWon: true},
			{ID: "sl", Name: "Perdido", Order: 2, IsLost: true},
		},
	}
}

type fakePipelines struct{ pl *pipelineentity.Pipeline }

func (f fakePipelines) Get(_ context.Context, id string) (*pipelineentity.Pipeline, error) {
	if f.pl != nil && f.pl.ID == id {
		cp := *f.pl
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (f fakePipelines) Default(_ context.Context) (*pipelineentity.Pipeline, error) {
	if f.pl != nil {
		cp := *f.pl
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}

type fakeConv struct {
	ref *contracts.ConversationRef
	err error
}

func (f fakeConv) Conversation(context.Context, string) (*contracts.ConversationRef, error) {
	return f.ref, f.err
}

type fakeContacts struct{ exists bool }

func (f fakeContacts) ContactExists(context.Context, string) (bool, error) { return f.exists, nil }

func newSvc() (*Service, *fakeRepo) {
	repo := newRepo()
	svc := New(repo, fakePipelines{pl: samplePipeline()}, nil)
	return svc, repo
}

func tenantCtx() context.Context { return shared.WithTenant(context.Background(), "t1") }
func authCtx(scope authz.SectorScope, sectors []string, user string) context.Context {
	return authz.WithAuthContext(tenantCtx(), authz.NewAuthContext("t1", user, authz.AllPermissions(), sectors, scope))
}

func TestCreate_DefaultsToFirstStageAndOpen(t *testing.T) {
	svc, _ := newSvc()
	d, err := svc.Create(tenantCtx(), contracts.CreateDeal{Title: "Cliente Acme", Value: 1000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if d.PipelineID != "p1" || d.StageID != "s1" {
		t.Errorf("must default to the tenant pipeline + first stage: %+v", d)
	}
	if d.Status != entity.StatusOpen || d.ClosedAt != nil {
		t.Errorf("a non-terminal stage must be open: status=%s closed=%v", d.Status, d.ClosedAt)
	}
	if d.StageChangedAt.IsZero() {
		t.Errorf("stage_changed_at must be set")
	}
	if d.Currency != "BRL" {
		t.Errorf("currency must default to BRL, got %q", d.Currency)
	}
}

func TestCreate_Validations(t *testing.T) {
	svc, _ := newSvc()
	if _, err := svc.Create(tenantCtx(), contracts.CreateDeal{Title: " "}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("empty title must be validation, got %v", err)
	}
	if _, err := svc.Create(tenantCtx(), contracts.CreateDeal{Title: "x", Value: -1}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("negative value must be validation, got %v", err)
	}
	if _, err := svc.Create(context.Background(), contracts.CreateDeal{Title: "x"}); err == nil {
		t.Error("missing tenant must error")
	}
}

func TestMoveStage_TerminalAndReopen(t *testing.T) {
	svc, _ := newSvc()
	ctx := tenantCtx()
	d, _ := svc.Create(ctx, contracts.CreateDeal{Title: "x", Value: 10})

	// Move to won → won + ClosedAt set.
	won, err := svc.MoveStage(ctx, d.ID, "sw")
	if err != nil {
		t.Fatalf("move won: %v", err)
	}
	if won.Status != entity.StatusWon || won.ClosedAt == nil {
		t.Errorf("won stage must close the deal: %+v", won)
	}
	prevChanged := won.StageChangedAt

	// Move back to a non-terminal stage → reopen (open, ClosedAt cleared).
	reopened, err := svc.MoveStage(ctx, d.ID, "s1")
	if err != nil {
		t.Fatalf("move back: %v", err)
	}
	if reopened.Status != entity.StatusOpen || reopened.ClosedAt != nil {
		t.Errorf("moving to a non-terminal stage must reopen: %+v", reopened)
	}
	if !reopened.StageChangedAt.After(prevChanged) && !reopened.StageChangedAt.Equal(prevChanged) {
		t.Errorf("stage_changed_at must be bumped on move")
	}
}

func TestMoveStage_RejectsForeignStage(t *testing.T) {
	svc, _ := newSvc()
	ctx := tenantCtx()
	d, _ := svc.Create(ctx, contracts.CreateDeal{Title: "x"})
	if _, err := svc.MoveStage(ctx, d.ID, "nope"); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("a stage outside the pipeline must be rejected, got %v", err)
	}
}

func TestCreateFromConversation_LinksContactAndConversation(t *testing.T) {
	svc, _ := newSvc()
	svc.SetConversationLookup(fakeConv{ref: &contracts.ConversationRef{ContactID: "ct1", SectorID: "sec1", AssignedTo: "u9"}})
	svc.SetContactChecker(fakeContacts{exists: true})

	d, err := svc.CreateFromConversation(tenantCtx(), contracts.CreateFromConversation{ConversationID: "cv1", Title: "Lead do chat"})
	if err != nil {
		t.Fatalf("create from conversation: %v", err)
	}
	if d.ContactID != "ct1" || d.SectorID != "sec1" || d.AssignedTo != "u9" {
		t.Errorf("must inherit contact/sector/assignee from the conversation: %+v", d)
	}
	if len(d.ConversationIDs) != 1 || d.ConversationIDs[0] != "cv1" {
		t.Errorf("must link the conversation: %+v", d.ConversationIDs)
	}
}

func TestLinkConversation_Idempotent(t *testing.T) {
	svc, _ := newSvc()
	ctx := tenantCtx()
	d, _ := svc.Create(ctx, contracts.CreateDeal{Title: "x"})

	d, _ = svc.LinkConversation(ctx, d.ID, "cv1")
	d, _ = svc.LinkConversation(ctx, d.ID, "cv1") // again
	if len(d.ConversationIDs) != 1 {
		t.Errorf("linking the same conversation twice must not duplicate: %+v", d.ConversationIDs)
	}
}

func TestMarkLost_MovesToLostStageAndRecordsReason(t *testing.T) {
	svc, _ := newSvc()
	ctx := tenantCtx()
	d, _ := svc.Create(ctx, contracts.CreateDeal{Title: "x"})

	lost, err := svc.MarkLost(ctx, d.ID, "preço")
	if err != nil {
		t.Fatalf("mark lost: %v", err)
	}
	if lost.Status != entity.StatusLost || lost.ClosedAt == nil {
		t.Errorf("must be lost + closed: %+v", lost)
	}
	if lost.StageID != "sl" {
		t.Errorf("must move to the pipeline's lost stage, got %q", lost.StageID)
	}
	if lost.LostReason != "preço" {
		t.Errorf("lost reason not recorded: %q", lost.LostReason)
	}
}

func TestStageHasDeals(t *testing.T) {
	svc, repo := newSvc()
	repo.stageCount = 2
	has, err := svc.StageHasDeals(tenantCtx(), "p1", "s1")
	if err != nil || !has {
		t.Errorf("expected has-deals true, got %v %v", has, err)
	}
	repo.stageCount = 0
	if has, _ := svc.StageHasDeals(tenantCtx(), "p1", "s1"); has {
		t.Errorf("expected has-deals false for an empty stage")
	}
}

func TestList_AppliesFilterAndVisibility(t *testing.T) {
	svc, repo := newSvc()
	// Restricted scope → visibility narrows to own/sectors.
	ctx := authCtx(authz.ScopeOwn, []string{"sec1"}, "u9")
	_, err := svc.List(ctx, contracts.ListFilter{PipelineID: "p1", Status: "open"}, shared.PageRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if repo.gotFilter.PipelineID != "p1" || repo.gotFilter.Status != "open" {
		t.Errorf("filter not forwarded: %+v", repo.gotFilter)
	}
	if repo.gotVis.All || repo.gotVis.UserID != "u9" || len(repo.gotVis.SectorIDs) != 1 {
		t.Errorf("own-scope visibility not applied: %+v", repo.gotVis)
	}

	// All-scope → sees everything.
	allCtx := authCtx(authz.ScopeAll, nil, "owner")
	if _, err := svc.List(allCtx, contracts.ListFilter{}, shared.PageRequest{Limit: 10}); err != nil {
		t.Fatalf("list all: %v", err)
	}
	if !repo.gotVis.All {
		t.Errorf("all-scope visibility must be All=true")
	}
}
