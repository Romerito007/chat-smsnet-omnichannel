package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	presenceentity "github.com/romerito007/chat-smsnet-omnichannel/domain/presence/entity"
	queueentity "github.com/romerito007/chat-smsnet-omnichannel/domain/queues/entity"
	queuerepo "github.com/romerito007/chat-smsnet-omnichannel/domain/queues/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/routing/contracts"
	sectorentity "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/entity"
	sectorrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── doubles ──────────────────────────────────────────────────────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeConvRepo struct {
	mu    sync.Mutex
	items map[string]*conventity.Conversation
}

func (r *fakeConvRepo) Create(_ context.Context, c *conventity.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *c
	r.items[c.ID] = &cp
	return nil
}
func (r *fakeConvRepo) Update(_ context.Context, c *conventity.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[c.ID]; !ok {
		return apperror.NotFound("not found")
	}
	cp := *c
	r.items[c.ID] = &cp
	return nil
}
func (r *fakeConvRepo) FindByID(ctx context.Context, id string) (*conventity.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenant, _ := shared.TenantFrom(ctx)
	if c, ok := r.items[id]; ok && c.TenantID == tenant {
		cp := *c
		return &cp, nil
	}
	return nil, apperror.NotFound("not found")
}
func (r *fakeConvRepo) FindOpenByContactChannel(ctx context.Context, contactID, channel string) (*conventity.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenant, _ := shared.TenantFrom(ctx)
	for _, c := range r.items {
		if c.TenantID == tenant && c.ContactID == contactID && c.Channel == channel && !c.Status.IsClosed() {
			cp := *c
			return &cp, nil
		}
	}
	return nil, apperror.NotFound("not found")
}

func (r *fakeConvRepo) List(ctx context.Context, f convcontracts.ListFilter, _ convcontracts.Visibility, _ shared.PageRequest) ([]*conventity.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenant, _ := shared.TenantFrom(ctx)
	var out []*conventity.Conversation
	for _, c := range r.items {
		if c.TenantID == tenant && (f.Status == "" || string(c.Status) == f.Status) {
			cp := *c
			out = append(out, &cp)
		}
	}
	return out, nil
}

type fakeEventRepo struct {
	mu    sync.Mutex
	items []*conventity.ConversationEvent
}

func (r *fakeEventRepo) Create(_ context.Context, e *conventity.ConversationEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, e)
	return nil
}
func (r *fakeEventRepo) ListByConversation(context.Context, string, shared.PageRequest) ([]*conventity.ConversationEvent, error) {
	return r.items, nil
}

func (r *fakeEventRepo) count(eventType string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.items {
		if e.Type == eventType {
			n++
		}
	}
	return n
}

type fakePresence struct {
	byUser map[string]*presenceentity.AgentPresence
}

func (p *fakePresence) Save(context.Context, *presenceentity.AgentPresence) error { return nil }
func (p *fakePresence) Get(_ context.Context, userID string) (*presenceentity.AgentPresence, error) {
	if pr, ok := p.byUser[userID]; ok {
		return pr, nil
	}
	return nil, apperror.NotFound("presence not found")
}
func (p *fakePresence) List(context.Context) ([]*presenceentity.AgentPresence, error) {
	return nil, nil
}

// fakeLoad is the live load counter (open assigned conversations per user).
type fakeLoad struct{ loads map[string]int }

func (l fakeLoad) CountOpenAssigned(_ context.Context, userID string) (int, error) {
	return l.loads[userID], nil
}

type fakeUsers struct {
	iamrepo.UserRepository
	byID map[string]*iamentity.User
}

func (r *fakeUsers) FindByID(_ context.Context, id string) (*iamentity.User, error) {
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, apperror.NotFound("not found")
}
func (r *fakeUsers) ListBySector(_ context.Context, sectorID string) ([]*iamentity.User, error) {
	var out []*iamentity.User
	for _, u := range r.byID {
		for _, s := range u.SectorIDs {
			if s == sectorID {
				out = append(out, u)
			}
		}
	}
	return out, nil
}

type fakeSectors struct {
	sectorrepo.SectorRepository
	exists map[string]bool
}

func (r *fakeSectors) FindByID(ctx context.Context, id string) (*sectorentity.Sector, error) {
	tenant, _ := shared.TenantFrom(ctx)
	if r.exists[id] {
		return &sectorentity.Sector{ID: id, TenantID: tenant}, nil
	}
	return nil, apperror.NotFound("not found")
}

type fakeQueues struct {
	queuerepo.QueueRepository
	byID map[string]*queueentity.Queue
}

func (r *fakeQueues) FindByID(_ context.Context, id string) (*queueentity.Queue, error) {
	if q, ok := r.byID[id]; ok {
		return q, nil
	}
	return nil, apperror.NotFound("not found")
}

// inMemLocker is a single-process NX lock for concurrency tests.
type inMemLocker struct {
	mu   sync.Mutex
	held map[string]bool
}

func newInMemLocker() *inMemLocker { return &inMemLocker{held: map[string]bool{}} }

func (l *inMemLocker) Acquire(_ context.Context, key string, _ time.Duration) (func(), bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held[key] {
		return func() {}, false, nil
	}
	l.held[key] = true
	return func() {
		l.mu.Lock()
		delete(l.held, key)
		l.mu.Unlock()
	}, true, nil
}

// denyLocker always fails to acquire.
type denyLocker struct{}

func (denyLocker) Acquire(context.Context, string, time.Duration) (func(), bool, error) {
	return func() {}, false, nil
}

// ── fixture ──────────────────────────────────────────────────────────────────

func adminCtx() context.Context {
	ctx := shared.WithTenant(context.Background(), "t1")
	return authz.WithAuthContext(ctx, authz.NewAuthContext("t1", "admin", nil, nil, authz.ScopeAll))
}

func agent(id, sector string, max int) *iamentity.User {
	return &iamentity.User{ID: id, TenantID: "t1", Status: iamentity.StatusActive, SectorIDs: []string{sector}, MaxConcurrentChats: max}
}

func presenceOf(id string, status presenceentity.Status, load, max int) *presenceentity.AgentPresence {
	return &presenceentity.AgentPresence{TenantID: "t1", UserID: id, Status: status, CurrentLoad: load, MaxConcurrentChats: max}
}

type fixture struct {
	svc    *Service
	convs  *fakeConvRepo
	events *fakeEventRepo
}

func newFixture(locker shared.Locker, users *fakeUsers, presence *fakePresence, loads map[string]int, convs map[string]*conventity.Conversation, queues map[string]*queueentity.Queue, sectors map[string]bool) fixture {
	cr := &fakeConvRepo{items: convs}
	er := &fakeEventRepo{}
	if loads == nil {
		loads = map[string]int{}
	}
	svc := New(cr, er, presence, fakeLoad{loads: loads}, users,
		&fakeSectors{exists: sectors},
		&fakeQueues{byID: queues},
		locker, shared.NoopPublisher{}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	return fixture{svc: svc, convs: cr, events: er}
}

func convNew(id, sector string) *conventity.Conversation {
	return &conventity.Conversation{ID: id, TenantID: "t1", SectorID: sector, Status: conventity.StatusNew, Channel: "wa", ContactID: "c"}
}

// ── pure scoring / eligibility ───────────────────────────────────────────────

func TestSortCandidates_LeastLoadedThenID(t *testing.T) {
	cands := []candidate{{"z", 2}, {"a", 1}, {"b", 1}, {"y", 0}}
	sortCandidates(cands)
	got := []string{cands[0].UserID, cands[1].UserID, cands[2].UserID, cands[3].UserID}
	want := []string{"y", "a", "b", "z"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestEligibleAgents_FiltersByAvailabilityAndCapacity(t *testing.T) {
	users := &fakeUsers{byID: map[string]*iamentity.User{
		"a": agent("a", "s1", 3),
		"b": agent("b", "s1", 3),
		"c": agent("c", "s1", 2),
		"d": agent("d", "s2", 3), // other sector
	}}
	presence := &fakePresence{byUser: map[string]*presenceentity.AgentPresence{
		"a": presenceOf("a", presenceentity.StatusAvailable, 2, 3),
		"b": presenceOf("b", presenceentity.StatusAvailable, 0, 3), // least loaded
		"c": presenceOf("c", presenceentity.StatusBusy, 0, 2),      // not available
		// d available but in s2; a different sector, excluded by ListBySector
		"d": presenceOf("d", presenceentity.StatusAvailable, 0, 3),
	}}
	fx := newFixture(shared.NoopLocker{}, users, presence, map[string]int{"a": 2, "b": 0}, map[string]*conventity.Conversation{}, nil, nil)

	cands, err := fx.svc.eligibleAgents(adminCtx(), "s1")
	if err != nil {
		t.Fatalf("eligible: %v", err)
	}
	if len(cands) != 2 {
		t.Fatalf("expected 2 eligible (a,b), got %d: %+v", len(cands), cands)
	}
	if cands[0].UserID != "b" {
		t.Errorf("least loaded should be b, got %s", cands[0].UserID)
	}
}

func TestEvaluateAgent_Reasons(t *testing.T) {
	users := &fakeUsers{byID: map[string]*iamentity.User{
		"in":     agent("in", "s1", 2),
		"out":    agent("out", "s2", 2),
		"busy":   agent("busy", "s1", 2),
		"full":   agent("full", "s1", 1),
		"absent": agent("absent", "s1", 2),
	}}
	presence := &fakePresence{byUser: map[string]*presenceentity.AgentPresence{
		"in":   presenceOf("in", presenceentity.StatusAvailable, 0, 2),
		"out":  presenceOf("out", presenceentity.StatusAvailable, 0, 2),
		"busy": presenceOf("busy", presenceentity.StatusBusy, 0, 2),
		"full": presenceOf("full", presenceentity.StatusAvailable, 1, 1),
		// absent has no presence record
	}}
	fx := newFixture(shared.NoopLocker{}, users, presence, map[string]int{"full": 1}, map[string]*conventity.Conversation{}, nil, nil)
	ctx := adminCtx()

	if err := fx.svc.evaluateAgent(ctx, "in", "s1"); err != nil {
		t.Errorf("eligible agent should pass: %v", err)
	}
	if c := apperror.From(fx.svc.evaluateAgent(ctx, "out", "s1")).Code; c != apperror.CodeValidation {
		t.Errorf("wrong-sector agent: want validation, got %s", c)
	}
	if c := apperror.From(fx.svc.evaluateAgent(ctx, "busy", "s1")).Code; c != apperror.CodeConflict {
		t.Errorf("busy agent: want conflict, got %s", c)
	}
	if c := apperror.From(fx.svc.evaluateAgent(ctx, "full", "s1")).Code; c != apperror.CodeConflict {
		t.Errorf("at-capacity agent: want conflict, got %s", c)
	}
	if c := apperror.From(fx.svc.evaluateAgent(ctx, "absent", "s1")).Code; c != apperror.CodeConflict {
		t.Errorf("offline agent: want conflict, got %s", c)
	}
}

// ── auto assignment ──────────────────────────────────────────────────────────

func TestAutoAssign_PicksLeastLoadedAndEmitsEvent(t *testing.T) {
	users := &fakeUsers{byID: map[string]*iamentity.User{
		"a": agent("a", "s1", 5),
		"b": agent("b", "s1", 5),
	}}
	presence := &fakePresence{byUser: map[string]*presenceentity.AgentPresence{
		"a": presenceOf("a", presenceentity.StatusAvailable, 3, 5),
		"b": presenceOf("b", presenceentity.StatusAvailable, 1, 5), // least loaded
	}}
	convs := map[string]*conventity.Conversation{"conv1": convNew("conv1", "s1")}
	fx := newFixture(shared.NoopLocker{}, users, presence, map[string]int{"a": 3, "b": 1}, convs, nil, nil)

	conv, err := fx.svc.AutoAssign(adminCtx(), "conv1")
	if err != nil {
		t.Fatalf("auto-assign: %v", err)
	}
	if conv.AssignedTo != "b" {
		t.Errorf("assigned to %q, want b (least loaded)", conv.AssignedTo)
	}
	if conv.Status != conventity.StatusAssigned {
		t.Errorf("status = %q, want assigned", conv.Status)
	}
	if fx.events.count(conventity.EventConversationAssigned) != 1 {
		t.Error("expected a conversation.assigned event")
	}
}

func TestAutoAssign_NoEligibleAgents(t *testing.T) {
	users := &fakeUsers{byID: map[string]*iamentity.User{"a": agent("a", "s1", 1)}}
	presence := &fakePresence{byUser: map[string]*presenceentity.AgentPresence{
		"a": presenceOf("a", presenceentity.StatusAway, 0, 1),
	}}
	convs := map[string]*conventity.Conversation{"conv1": convNew("conv1", "s1")}
	fx := newFixture(shared.NoopLocker{}, users, presence, nil, convs, nil, nil)

	if _, err := fx.svc.AutoAssign(adminCtx(), "conv1"); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("expected conflict (no eligible), got %v", err)
	}
}

func TestAssign_ManualClosedConversationRejected(t *testing.T) {
	users := &fakeUsers{byID: map[string]*iamentity.User{"a": agent("a", "s1", 2)}}
	presence := &fakePresence{byUser: map[string]*presenceentity.AgentPresence{"a": presenceOf("a", presenceentity.StatusAvailable, 0, 2)}}
	closed := convNew("conv1", "s1")
	closed.Status = conventity.StatusClosed
	fx := newFixture(shared.NoopLocker{}, users, presence, nil, map[string]*conventity.Conversation{"conv1": closed}, nil, nil)

	if _, err := fx.svc.Assign(adminCtx(), "conv1", "a"); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("expected conflict assigning a closed conversation, got %v", err)
	}
}

// ── lock ─────────────────────────────────────────────────────────────────────

func TestAssign_LockNotAcquired(t *testing.T) {
	users := &fakeUsers{byID: map[string]*iamentity.User{"a": agent("a", "s1", 2)}}
	presence := &fakePresence{byUser: map[string]*presenceentity.AgentPresence{"a": presenceOf("a", presenceentity.StatusAvailable, 0, 2)}}
	convs := map[string]*conventity.Conversation{"conv1": convNew("conv1", "s1")}
	fx := newFixture(denyLocker{}, users, presence, nil, convs, nil, nil)

	if _, err := fx.svc.Assign(adminCtx(), "conv1", "a"); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("expected conflict when lock is unavailable, got %v", err)
	}
}

func TestAutoAssign_ConcurrentSingleWinner(t *testing.T) {
	users := &fakeUsers{byID: map[string]*iamentity.User{
		"a": agent("a", "s1", 5),
		"b": agent("b", "s1", 5),
	}}
	presence := &fakePresence{byUser: map[string]*presenceentity.AgentPresence{
		"a": presenceOf("a", presenceentity.StatusAvailable, 0, 5),
		"b": presenceOf("b", presenceentity.StatusAvailable, 1, 5),
	}}
	convs := map[string]*conventity.Conversation{"conv1": convNew("conv1", "s1")}
	fx := newFixture(newInMemLocker(), users, presence, nil, convs, nil, nil)

	const n = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := fx.svc.AutoAssign(adminCtx(), "conv1"); err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if success != 1 {
		t.Fatalf("expected exactly one successful assignment, got %d", success)
	}
	if fx.events.count(conventity.EventConversationAssigned) != 1 {
		t.Errorf("expected exactly one assigned event, got %d", fx.events.count(conventity.EventConversationAssigned))
	}
}

// ── transfer & enqueue ───────────────────────────────────────────────────────

func TestTransfer_ChangesSectorAndAgent(t *testing.T) {
	users := &fakeUsers{byID: map[string]*iamentity.User{"b": agent("b", "s2", 5)}}
	presence := &fakePresence{byUser: map[string]*presenceentity.AgentPresence{"b": presenceOf("b", presenceentity.StatusAvailable, 0, 5)}}
	conv := convNew("conv1", "s1")
	conv.AssignedTo = "a"
	conv.Status = conventity.StatusAssigned
	fx := newFixture(shared.NoopLocker{}, users, presence, nil, map[string]*conventity.Conversation{"conv1": conv}, nil, map[string]bool{"s2": true})

	out, err := fx.svc.Transfer(adminCtx(), "conv1", contracts.TransferCommand{SectorID: "s2", AgentID: "b"})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if out.SectorID != "s2" || out.AssignedTo != "b" || out.Status != conventity.StatusAssigned {
		t.Errorf("unexpected conversation after transfer: %+v", out)
	}
	if fx.events.count(conventity.EventConversationTransferred) != 1 {
		t.Error("expected a conversation.transferred event")
	}
}

func TestEnqueue_SetsQueueAndSectorFromQueue(t *testing.T) {
	queues := map[string]*queueentity.Queue{
		"q1": {ID: "q1", TenantID: "t1", SectorID: "s9", Name: "Q", Strategy: queueentity.StrategyManual},
	}
	conv := convNew("conv1", "s1")
	fx := newFixture(shared.NoopLocker{}, &fakeUsers{byID: map[string]*iamentity.User{}}, &fakePresence{byUser: map[string]*presenceentity.AgentPresence{}}, nil, map[string]*conventity.Conversation{"conv1": conv}, queues, nil)

	out, err := fx.svc.Enqueue(adminCtx(), "conv1", contracts.EnqueueCommand{QueueID: "q1"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if out.QueueID != "q1" || out.SectorID != "s9" || out.Status != conventity.StatusQueued {
		t.Errorf("unexpected conversation after enqueue: %+v", out)
	}
	if out.AssignedTo != "" {
		t.Errorf("enqueue should clear assignee, got %q", out.AssignedTo)
	}
	if fx.events.count(conventity.EventConversationEnqueued) != 1 {
		t.Error("expected a conversation.enqueued event")
	}
}
