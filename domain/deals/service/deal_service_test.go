package service

import (
	"context"
	"testing"
	"time"

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

	// sales-metrics canned returns + capture (used by metrics_service_test.go).
	openByStage   []contracts.FunnelStage
	closedTotals  map[string]contracts.CountValue            // status → totals
	openByAgent   map[string]contracts.CountValue            // agent → totals
	closedByAgent map[string]map[string]contracts.CountValue // status → agent → totals
	avgClose      float64
	avgWonCount   int
	stageDwell    []contracts.StageDwell
	stalled       []*entity.Deal
	gotSalesVis   contracts.Visibility
	gotSalesF     contracts.SalesFilter
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
func (r *fakeRepo) FindByConversation(_ context.Context, conversationID string) ([]*entity.Deal, error) {
	var out []*entity.Deal
	for _, d := range r.byID {
		if d.HasConversation(conversationID) {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeRepo) OpenByStage(_ context.Context, f contracts.SalesFilter, vis contracts.Visibility) ([]contracts.FunnelStage, error) {
	r.gotSalesVis, r.gotSalesF = vis, f
	return r.openByStage, nil
}
func (r *fakeRepo) ClosedTotals(_ context.Context, status string, f contracts.SalesFilter, vis contracts.Visibility) (contracts.CountValue, error) {
	r.gotSalesVis, r.gotSalesF = vis, f
	return r.closedTotals[status], nil
}
func (r *fakeRepo) OpenByAgent(_ context.Context, f contracts.SalesFilter, vis contracts.Visibility) (map[string]contracts.CountValue, error) {
	r.gotSalesVis, r.gotSalesF = vis, f
	return r.openByAgent, nil
}
func (r *fakeRepo) ClosedByAgent(_ context.Context, status string, f contracts.SalesFilter, vis contracts.Visibility) (map[string]contracts.CountValue, error) {
	r.gotSalesVis, r.gotSalesF = vis, f
	return r.closedByAgent[status], nil
}
func (r *fakeRepo) AvgCloseSeconds(_ context.Context, f contracts.SalesFilter, vis contracts.Visibility) (float64, int, error) {
	r.gotSalesVis, r.gotSalesF = vis, f
	return r.avgClose, r.avgWonCount, nil
}
func (r *fakeRepo) StageDwell(_ context.Context, _ time.Time, f contracts.SalesFilter, vis contracts.Visibility) ([]contracts.StageDwell, error) {
	r.gotSalesVis, r.gotSalesF = vis, f
	return r.stageDwell, nil
}
func (r *fakeRepo) StalledOpen(_ context.Context, _ time.Time, _ int, f contracts.SalesFilter, vis contracts.Visibility) ([]*entity.Deal, error) {
	r.gotSalesVis, r.gotSalesF = vis, f
	return r.stalled, nil
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

type capturingAuditor struct{ entries []shared.AuditEntry }

func (a *capturingAuditor) Record(_ context.Context, e shared.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

type capturingNotifier struct{ inputs []shared.NotifyInput }

func (n *capturingNotifier) Notify(_ context.Context, in shared.NotifyInput) {
	n.inputs = append(n.inputs, in)
}

// newAutoSvc builds a deal service with a capturing auditor + notifier for the
// automation-move tests.
func newAutoSvc() (*Service, *fakeRepo, *capturingAuditor, *capturingNotifier) {
	repo := newRepo()
	svc := New(repo, fakePipelines{pl: samplePipeline()}, nil)
	aud, not := &capturingAuditor{}, &capturingNotifier{}
	svc.SetAuditor(aud)
	svc.SetNotifier(not)
	return svc, repo, aud, not
}

func seedDeal(repo *fakeRepo, d *entity.Deal) { repo.byID[d.ID] = d }

type capturedEvent struct {
	topic string
	event string
	data  contracts.DealEvent
}

type fakePublisher struct{ events []capturedEvent }

func (p *fakePublisher) Publish(_ context.Context, topic, event string, data any) error {
	de, _ := data.(contracts.DealEvent)
	p.events = append(p.events, capturedEvent{topic: topic, event: event, data: de})
	return nil
}

func (p *fakePublisher) byEvent(name string) []capturedEvent {
	var out []capturedEvent
	for _, e := range p.events {
		if e.event == name {
			out = append(out, e)
		}
	}
	return out
}

func (p *fakePublisher) topicsFor(name string) map[string]bool {
	out := map[string]bool{}
	for _, e := range p.byEvent(name) {
		out[e.topic] = true
	}
	return out
}

func movedBys(entries []shared.AuditEntry) []string {
	var out []string
	for _, e := range entries {
		if e.Action == "deal.stage_moved" {
			out = append(out, e.Data["moved_by"].(string))
		}
	}
	return out
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

func TestAutomationMoveDealStage_MovesLinkedDealAndNotifiesSeller(t *testing.T) {
	svc, repo, aud, not := newAutoSvc()
	seedDeal(repo, &entity.Deal{
		ID: "d1", TenantID: "t1", PipelineID: "p1", StageID: "s1", Status: entity.StatusOpen,
		AssignedTo: "u1", Title: "Acme", ConversationIDs: []string{"cv1"},
	})

	if err := svc.AutomationMoveDealStage(tenantCtx(), "cv1", "p1", "sw"); err != nil {
		t.Fatalf("move: %v", err)
	}
	moved := repo.byID["d1"]
	if moved.StageID != "sw" || moved.Status != entity.StatusWon || moved.ClosedAt == nil {
		t.Errorf("deal not moved to the won target stage: %+v", moved)
	}
	// Audit records the move tagged moved_by=automation.
	if got := movedBys(aud.entries); len(got) != 1 || got[0] != "automation" {
		t.Errorf("expected one move audited as automation, got %+v", got)
	}
	// The seller is notified in-app.
	if len(not.inputs) != 1 || not.inputs[0].UserID != "u1" || not.inputs[0].Type != "deal.stage_moved_by_automation" {
		t.Errorf("seller not notified correctly: %+v", not.inputs)
	}
}

func TestAutomationMoveDealStage_Idempotent(t *testing.T) {
	svc, repo, aud, not := newAutoSvc()
	seedDeal(repo, &entity.Deal{
		ID: "d1", TenantID: "t1", PipelineID: "p1", StageID: "s1", Status: entity.StatusOpen,
		AssignedTo: "u1", Title: "Acme", ConversationIDs: []string{"cv1"},
	})
	// Target is the deal's CURRENT stage → nothing happens.
	if err := svc.AutomationMoveDealStage(tenantCtx(), "cv1", "p1", "s1"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if len(movedBys(aud.entries)) != 0 || len(not.inputs) != 0 {
		t.Errorf("a no-op move must not audit or notify: audits=%+v notifs=%+v", aud.entries, not.inputs)
	}
}

func TestAutomationMoveDealStage_NoLinkedDealDoesNothing(t *testing.T) {
	svc, repo, aud, not := newAutoSvc()
	seedDeal(repo, &entity.Deal{ID: "d1", TenantID: "t1", PipelineID: "p1", StageID: "s1", Status: entity.StatusOpen, ConversationIDs: []string{"cv1"}})

	// A conversation with no linked deal → no move, no create, no error.
	if err := svc.AutomationMoveDealStage(tenantCtx(), "other-conv", "p1", "sw"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if repo.byID["d1"].StageID != "s1" {
		t.Errorf("an unrelated deal must not move")
	}
	if len(movedBys(aud.entries)) != 0 || len(not.inputs) != 0 {
		t.Errorf("nothing should be audited/notified: audits=%+v notifs=%+v", aud.entries, not.inputs)
	}
}

func TestAutomationMoveDealStage_SkipsDealInAnotherPipeline(t *testing.T) {
	svc, repo, aud, _ := newAutoSvc()
	// The linked deal belongs to a DIFFERENT pipeline than the rule's target.
	seedDeal(repo, &entity.Deal{ID: "d1", TenantID: "t1", PipelineID: "p2", StageID: "x1", Status: entity.StatusOpen, ConversationIDs: []string{"cv1"}})

	if err := svc.AutomationMoveDealStage(tenantCtx(), "cv1", "p1", "sw"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if repo.byID["d1"].StageID != "x1" {
		t.Errorf("a deal in another pipeline must be left untouched: %+v", repo.byID["d1"])
	}
	if len(movedBys(aud.entries)) != 0 {
		t.Errorf("no move should be audited, got %+v", aud.entries)
	}
}

func TestAutomationMoveDealStage_UnassignedDealMovesWithoutNotify(t *testing.T) {
	svc, repo, aud, not := newAutoSvc()
	seedDeal(repo, &entity.Deal{ID: "d1", TenantID: "t1", PipelineID: "p1", StageID: "s1", Status: entity.StatusOpen, Title: "Acme", ConversationIDs: []string{"cv1"}})

	if err := svc.AutomationMoveDealStage(tenantCtx(), "cv1", "p1", "sw"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if repo.byID["d1"].StageID != "sw" {
		t.Errorf("deal should still move when unassigned")
	}
	if len(movedBys(aud.entries)) != 1 {
		t.Errorf("the move should still be audited")
	}
	if len(not.inputs) != 0 {
		t.Errorf("no seller → no notification, got %+v", not.inputs)
	}
}

func TestMoveStage_PublishesStageChangedRealtime(t *testing.T) {
	repo := newRepo()
	svc := New(repo, fakePipelines{pl: samplePipeline()}, nil)
	pub := &fakePublisher{}
	svc.SetPublisher(pub)
	seedDeal(repo, &entity.Deal{
		ID: "d1", TenantID: "t1", PipelineID: "p1", StageID: "s1", Status: entity.StatusOpen,
		SectorID: "sec1", AssignedTo: "u1", Title: "Acme", ConversationIDs: []string{"cv1"},
	})

	if _, err := svc.MoveStage(tenantCtx(), "d1", "sw"); err != nil {
		t.Fatalf("move: %v", err)
	}
	got := pub.byEvent(contracts.RealtimeDealStageChanged)
	if len(got) == 0 {
		t.Fatalf("expected a deal.stage_changed event, got none")
	}
	// Visibility rooms: all-scope (unassigned), the deal's sector, and the seller.
	want := map[string]bool{
		shared.TopicUnassigned("t1"):    true,
		shared.TopicInbox("t1", "sec1"): true,
		shared.TopicUser("t1", "u1"):    true,
	}
	if topics := pub.topicsFor(contracts.RealtimeDealStageChanged); len(topics) != len(want) {
		t.Errorf("wrong topics: got %v want %v", topics, want)
	} else {
		for tp := range want {
			if !topics[tp] {
				t.Errorf("missing topic %q in %v", tp, topics)
			}
		}
	}
	p := got[0].data
	if p.DealID != "d1" || p.PipelineID != "p1" || p.FromStageID != "s1" || p.ToStageID != "sw" {
		t.Errorf("payload ids wrong: %+v", p)
	}
	if p.Status != string(entity.StatusWon) || p.MovedBy != "user" || p.AssignedTo != "u1" {
		t.Errorf("payload status/moved_by/assigned_to wrong: %+v", p)
	}
}

func TestAutomationMoveDealStage_PublishesStageChangedAsAutomation(t *testing.T) {
	repo := newRepo()
	svc := New(repo, fakePipelines{pl: samplePipeline()}, nil)
	pub := &fakePublisher{}
	svc.SetPublisher(pub)
	seedDeal(repo, &entity.Deal{
		ID: "d1", TenantID: "t1", PipelineID: "p1", StageID: "s1", Status: entity.StatusOpen,
		AssignedTo: "u1", Title: "Acme", ConversationIDs: []string{"cv1"},
	})

	if err := svc.AutomationMoveDealStage(tenantCtx(), "cv1", "p1", "sw"); err != nil {
		t.Fatalf("move: %v", err)
	}
	got := pub.byEvent(contracts.RealtimeDealStageChanged)
	if len(got) == 0 {
		t.Fatalf("expected a deal.stage_changed event from the automation move")
	}
	p := got[0].data
	if p.FromStageID != "s1" || p.ToStageID != "sw" || p.MovedBy != "automation" {
		t.Errorf("automation stage-change payload wrong: %+v", p)
	}
	// A sector-less deal reaches the all-scope room + the seller (no sector room).
	topics := pub.topicsFor(contracts.RealtimeDealStageChanged)
	if !topics[shared.TopicUnassigned("t1")] || !topics[shared.TopicUser("t1", "u1")] {
		t.Errorf("expected unassigned + user rooms, got %v", topics)
	}
	if topics[shared.TopicInbox("t1", "")] {
		t.Errorf("a sector-less deal must not publish to an empty sector room: %v", topics)
	}
}

func TestMoveStage_Idempotent_NoChangeStillPublishes(t *testing.T) {
	// MoveStage to a different stage always publishes; this guards the from→to wiring.
	repo := newRepo()
	svc := New(repo, fakePipelines{pl: samplePipeline()}, nil)
	pub := &fakePublisher{}
	svc.SetPublisher(pub)
	seedDeal(repo, &entity.Deal{ID: "d1", TenantID: "t1", PipelineID: "p1", StageID: "s1", Status: entity.StatusOpen})

	if _, err := svc.MoveStage(tenantCtx(), "d1", "sw"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if len(pub.byEvent(contracts.RealtimeDealStageChanged)) == 0 {
		t.Errorf("a manual move must emit deal.stage_changed")
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
