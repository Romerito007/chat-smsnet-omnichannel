package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/presence/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── doubles ──────────────────────────────────────────────────────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// fakeStore models the Redis store: a cached effective-status hash (items) plus a
// per-user set of live socket ids (live).
type fakeStore struct {
	items map[string]*entity.AgentPresence // key tenant|user → cached effective
	live  map[string]map[string]struct{}   // key tenant|user → set of clientIDs
}

func newFakeStore() *fakeStore {
	return &fakeStore{items: map[string]*entity.AgentPresence{}, live: map[string]map[string]struct{}{}}
}

func key(tenant, user string) string { return tenant + "|" + user }

func (s *fakeStore) Save(_ context.Context, p *entity.AgentPresence) error {
	cp := *p
	s.items[key(p.TenantID, p.UserID)] = &cp
	return nil
}
func (s *fakeStore) Connect(ctx context.Context, userID, clientID string) (bool, error) {
	tenant, _ := shared.TenantFrom(ctx)
	k := key(tenant, userID)
	before := len(s.live[k])
	if s.live[k] == nil {
		s.live[k] = map[string]struct{}{}
	}
	s.live[k][clientID] = struct{}{}
	return before == 0, nil
}
func (s *fakeStore) Heartbeat(ctx context.Context, userID, clientID string) error {
	_, err := s.Connect(ctx, userID, clientID)
	return err
}
func (s *fakeStore) Disconnect(ctx context.Context, userID, clientID string) (bool, error) {
	tenant, _ := shared.TenantFrom(ctx)
	k := key(tenant, userID)
	delete(s.live[k], clientID)
	if len(s.live[k]) == 0 {
		delete(s.live, k)
		return true, nil
	}
	return false, nil
}
func (s *fakeStore) HasLiveSocket(ctx context.Context, userID string) (bool, error) {
	tenant, _ := shared.TenantFrom(ctx)
	return len(s.live[key(tenant, userID)]) > 0, nil
}
func (s *fakeStore) Remove(ctx context.Context, userID string) error {
	tenant, _ := shared.TenantFrom(ctx)
	delete(s.items, key(tenant, userID))
	delete(s.live, key(tenant, userID))
	return nil
}
func (s *fakeStore) Get(ctx context.Context, userID string) (*entity.AgentPresence, error) {
	tenant, _ := shared.TenantFrom(ctx)
	if p, ok := s.items[key(tenant, userID)]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, apperror.NotFound("presence not found")
}
func (s *fakeStore) List(ctx context.Context) ([]*entity.AgentPresence, error) {
	tenant, _ := shared.TenantFrom(ctx)
	var out []*entity.AgentPresence
	for _, p := range s.items {
		if p.TenantID == tenant {
			cp := *p
			out = append(out, &cp)
		}
	}
	return out, nil
}

type fakeLoad struct {
	n          int
	loads      map[string]int
	countCalls *int
	batchCalls *int
}

func (l fakeLoad) CountOpenAssigned(_ context.Context, userID string) (int, error) {
	if l.countCalls != nil {
		*l.countCalls++
	}
	if l.loads != nil {
		return l.loads[userID], nil
	}
	return l.n, nil
}
func (l fakeLoad) OpenAssignedLoads(context.Context) (map[string]int, error) {
	if l.batchCalls != nil {
		*l.batchCalls++
	}
	return l.loads, nil
}

type fakeUsers struct {
	iamrepo.UserRepository
	users map[string]*iamentity.User
}

func (r *fakeUsers) FindByID(ctx context.Context, id string) (*iamentity.User, error) {
	tenant, _ := shared.TenantFrom(ctx)
	if u, ok := r.users[id]; ok && u.TenantID == tenant {
		cp := *u
		return &cp, nil
	}
	return nil, apperror.NotFound("resource not found")
}

func (r *fakeUsers) ListBySector(_ context.Context, sectorID string) ([]*iamentity.User, error) {
	var out []*iamentity.User
	for _, u := range r.users {
		for _, s := range u.SectorIDs {
			if s == sectorID {
				out = append(out, u)
			}
		}
	}
	return out, nil
}

// SetPresenceSettings mutates the durable availability/auto_offline on the user doc.
func (r *fakeUsers) SetPresenceSettings(_ context.Context, userID string, availability *string, autoOffline *bool) error {
	u, ok := r.users[userID]
	if !ok {
		return apperror.NotFound("resource not found")
	}
	if availability != nil {
		u.PresenceAvailability = *availability
	}
	if autoOffline != nil {
		u.AutoOffline = autoOffline
	}
	return nil
}

type capturedEvent struct {
	topic string
	event string
}

type fakePublisher struct{ events []capturedEvent }

func (p *fakePublisher) Publish(_ context.Context, topic, event string, _ any) error {
	p.events = append(p.events, capturedEvent{topic, event})
	return nil
}

// ── fixture ──────────────────────────────────────────────────────────────────

func actorCtx(tenant, userID string, perms ...authz.Permission) context.Context {
	ctx := shared.WithTenant(context.Background(), tenant)
	ac := authz.NewAuthContext(tenant, userID, perms, nil, authz.ScopeAll)
	return authz.WithAuthContext(ctx, ac)
}

func newPresenceService(load int, users map[string]*iamentity.User) (*Service, *fakeStore, *fakePublisher) {
	store := newFakeStore()
	pub := &fakePublisher{}
	loads := make(map[string]int, len(users))
	for id := range users {
		loads[id] = load
	}
	svc := New(store, fakeLoad{n: load, loads: loads}, &fakeUsers{users: users}, pub, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	return svc, store, pub
}

func userWith(tenant, id string, sectors []string, maxChats int) map[string]*iamentity.User {
	return map[string]*iamentity.User{
		id: {ID: id, TenantID: tenant, SectorIDs: sectors, MaxConcurrentChats: maxChats},
	}
}

func effective(t *testing.T, store *fakeStore, ctx context.Context, userID string) entity.Status {
	t.Helper()
	p, err := store.Get(ctx, userID)
	if err != nil {
		t.Fatalf("get presence %s: %v", userID, err)
	}
	return p.Status
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestSetStatus_PersistsAvailabilityAndPublishes(t *testing.T) {
	svc, store, pub := newPresenceService(2, userWith("t1", "u1", []string{"s1"}, 5))
	ctx := actorCtx("t1", "u1")
	// A live socket so availability=online resolves to effective online.
	_ = svc.Connected(ctx, "u1", "c1")
	pub.events = nil

	p, err := svc.SetStatus(ctx, "", entity.StatusOnline)
	if err != nil {
		t.Fatalf("set status: %v", err)
	}
	if p.Status != entity.StatusOnline || p.Availability != "online" {
		t.Errorf("effective/availability wrong: %+v", p)
	}
	if p.CurrentLoad != 2 {
		t.Errorf("current_load = %d, want 2", p.CurrentLoad)
	}
	if store.items[key("t1", "u1")] == nil {
		t.Error("effective not cached")
	}
	if len(pub.events) != 2 {
		t.Fatalf("expected 2 events (presence + user), got %d", len(pub.events))
	}
}

func TestSetStatus_InvalidStatus(t *testing.T) {
	svc, _, _ := newPresenceService(0, userWith("t1", "u1", []string{"s1"}, 5))
	ctx := actorCtx("t1", "u1")
	if _, err := svc.SetStatus(ctx, "", entity.Status("teleporting")); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation_error, got %v", err)
	}
}

func TestSetStatus_CannotChangeOthersWithoutPermission(t *testing.T) {
	users := map[string]*iamentity.User{
		"u1": {ID: "u1", TenantID: "t1"},
		"u2": {ID: "u2", TenantID: "t1"},
	}
	svc, _, _ := newPresenceService(0, users)

	ctx := actorCtx("t1", "u1")
	if _, err := svc.SetStatus(ctx, "u2", entity.StatusOnline); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expected forbidden, got %v", err)
	}
	sup := actorCtx("t1", "u1", authz.UserManage)
	if _, err := svc.SetStatus(sup, "u2", entity.StatusOnline); err != nil {
		t.Errorf("supervisor setting other status: %v", err)
	}
}

// offline manual + socket vivo → offline (a live socket never overrides offline).
func TestEffective_ManualOfflineWithLiveSocket(t *testing.T) {
	svc, store, _ := newPresenceService(0, userWith("t1", "u1", []string{"s1"}, 5))
	ctx := actorCtx("t1", "u1")
	if _, err := svc.SetStatus(ctx, "", entity.StatusOffline); err != nil {
		t.Fatalf("offline: %v", err)
	}
	_ = svc.Connected(ctx, "u1", "c1")
	if got := effective(t, store, ctx, "u1"); got != entity.StatusOffline {
		t.Errorf("manual offline + live socket must be offline, got %q", got)
	}
}

// away manual + reconnect → away (sticky, socket-independent).
func TestEffective_ManualAwaySticky(t *testing.T) {
	svc, store, _ := newPresenceService(0, userWith("t1", "u1", []string{"s1"}, 5))
	ctx := actorCtx("t1", "u1")
	if _, err := svc.SetStatus(ctx, "", entity.StatusAway); err != nil {
		t.Fatalf("away: %v", err)
	}
	_ = svc.Connected(ctx, "u1", "c1")
	if got := effective(t, store, ctx, "u1"); got != entity.StatusAway {
		t.Errorf("manual away must stay away with a socket, got %q", got)
	}
	// drop + reconnect → still away.
	_, _ = svc.Disconnected(ctx, "u1", "c1")
	_ = svc.Vanished(ctx, "u1")
	_ = svc.Connected(ctx, "u1", "c2")
	if got := effective(t, store, ctx, "u1"); got != entity.StatusAway {
		t.Errorf("away must survive reconnect, got %q", got)
	}
}

// online + auto_offline ON + last tab closes → offline.
func TestEffective_OnlineAutoOfflineOn_LastTabClosesOffline(t *testing.T) {
	svc, store, _ := newPresenceService(0, userWith("t1", "u1", []string{"s1"}, 5))
	ctx := actorCtx("t1", "u1")
	_ = svc.Connected(ctx, "u1", "c1") // availability online (default), auto_offline on (default)
	if got := effective(t, store, ctx, "u1"); got != entity.StatusOnline {
		t.Fatalf("first socket must be online, got %q", got)
	}
	last, _ := svc.Disconnected(ctx, "u1", "c1")
	if !last {
		t.Fatal("closing the only socket must report lastGone")
	}
	_ = svc.Vanished(ctx, "u1")
	if got := effective(t, store, ctx, "u1"); got != entity.StatusOffline {
		t.Errorf("online + auto_offline ON + last tab → offline, got %q", got)
	}
}

// online + auto_offline OFF + last tab closes → stays online.
func TestEffective_OnlineAutoOfflineOff_StaysOnline(t *testing.T) {
	svc, store, _ := newPresenceService(0, userWith("t1", "u1", []string{"s1"}, 5))
	ctx := actorCtx("t1", "u1")
	if _, err := svc.SetAutoOffline(ctx, "", false); err != nil {
		t.Fatalf("auto_offline off: %v", err)
	}
	_ = svc.Connected(ctx, "u1", "c1")
	_, _ = svc.Disconnected(ctx, "u1", "c1")
	_ = svc.Vanished(ctx, "u1")
	if got := effective(t, store, ctx, "u1"); got != entity.StatusOnline {
		t.Errorf("online + auto_offline OFF + last tab → stays online, got %q", got)
	}
}

// online + 2 tabs, close 1 → still online (last socket not yet gone).
func TestEffective_MultiTabKeepsOnline(t *testing.T) {
	svc, store, _ := newPresenceService(0, userWith("t1", "u1", []string{"s1"}, 5))
	ctx := actorCtx("t1", "u1")
	_ = svc.Connected(ctx, "u1", "c1")
	_ = svc.Connected(ctx, "u1", "c2")
	last, _ := svc.Disconnected(ctx, "u1", "c1")
	if last {
		t.Fatal("closing one of two tabs must not be lastGone")
	}
	// Even a stray recompute keeps online (a socket still lives).
	_ = svc.Vanished(ctx, "u1")
	if got := effective(t, store, ctx, "u1"); got != entity.StatusOnline {
		t.Errorf("with a tab still open the agent stays online, got %q", got)
	}
}

// offline persisted (durable on the user doc) + new login → offline (not promoted).
func TestEffective_PersistedOfflineSurvivesLogin(t *testing.T) {
	off := "offline"
	users := map[string]*iamentity.User{
		"u1": {ID: "u1", TenantID: "t1", SectorIDs: []string{"s1"}, PresenceAvailability: off},
	}
	svc, store, _ := newPresenceService(0, users)
	ctx := actorCtx("t1", "u1")
	_ = svc.Connected(ctx, "u1", "c1") // a fresh login
	if got := effective(t, store, ctx, "u1"); got != entity.StatusOffline {
		t.Errorf("a persisted offline must not be promoted on login, got %q", got)
	}
}

// A never-configured agent defaults to online on its first socket.
func TestEffective_DefaultOnlineOnFirstSocket(t *testing.T) {
	svc, store, _ := newPresenceService(0, userWith("t1", "u1", []string{"s1"}, 5))
	ctx := actorCtx("t1", "u1")
	_ = svc.Connected(ctx, "u1", "c1")
	if got := effective(t, store, ctx, "u1"); got != entity.StatusOnline {
		t.Errorf("a fresh login defaults to online, got %q", got)
	}
}

func TestList_ReturnsEffectivePlusRawSettings(t *testing.T) {
	svc, _, _ := newPresenceService(4, userWith("t1", "u1", []string{"s1"}, 10))
	ctx := actorCtx("t1", "u1")
	_ = svc.Connected(ctx, "u1", "c1")

	list, err := svc.List(ctx, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(list))
	}
	row := list[0]
	if row.Status != entity.StatusOnline || row.Availability != "online" || !row.AutoOffline {
		t.Errorf("list row must carry effective + raw availability + auto_offline: %+v", row)
	}
	if row.CurrentLoad != 4 {
		t.Errorf("load recomputed from aggregation: %d", row.CurrentLoad)
	}
}
