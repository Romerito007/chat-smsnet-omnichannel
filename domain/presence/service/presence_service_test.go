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

type fakeStore struct {
	items map[string]*entity.AgentPresence // key: tenant|user
}

func newFakeStore() *fakeStore { return &fakeStore{items: map[string]*entity.AgentPresence{}} }

func key(tenant, user string) string { return tenant + "|" + user }

func (s *fakeStore) Save(ctx context.Context, p *entity.AgentPresence) error {
	cp := *p
	s.items[key(p.TenantID, p.UserID)] = &cp
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

type fakeLoad struct{ n int }

func (l fakeLoad) CountOpenAssigned(context.Context, string) (int, error) { return l.n, nil }

type fakeUsers struct {
	iamrepo.UserRepository
	users map[string]*iamentity.User
}

func (r *fakeUsers) FindByID(ctx context.Context, id string) (*iamentity.User, error) {
	tenant, _ := shared.TenantFrom(ctx)
	if u, ok := r.users[id]; ok && u.TenantID == tenant {
		return u, nil
	}
	return nil, apperror.NotFound("resource not found")
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
	svc := New(store, fakeLoad{n: load}, &fakeUsers{users: users}, pub, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	return svc, store, pub
}

func userWith(tenant, id string, sectors []string, maxChats int) map[string]*iamentity.User {
	return map[string]*iamentity.User{
		id: {ID: id, TenantID: tenant, SectorIDs: sectors, MaxConcurrentChats: maxChats},
	}
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestSetStatus_OwnStatusPublishesEvent(t *testing.T) {
	svc, store, pub := newPresenceService(2, userWith("t1", "u1", []string{"s1"}, 5))
	ctx := actorCtx("t1", "u1")

	p, err := svc.SetStatus(ctx, "", entity.StatusOnline)
	if err != nil {
		t.Fatalf("set status: %v", err)
	}
	if p.Status != entity.StatusOnline {
		t.Errorf("status = %q", p.Status)
	}
	if p.CurrentLoad != 2 {
		t.Errorf("current_load = %d, want 2 (from load counter)", p.CurrentLoad)
	}
	if store.items[key("t1", "u1")] == nil {
		t.Error("presence not stored in Redis-equivalent")
	}
	if len(pub.events) != 2 {
		t.Fatalf("expected 2 events (presence + user topic), got %d", len(pub.events))
	}
	for _, e := range pub.events {
		if e.event != "agent.presence_changed" {
			t.Errorf("unexpected event name %q", e.event)
		}
	}
}

func TestSetStatus_AvailableRequiresOnline(t *testing.T) {
	svc, _, _ := newPresenceService(0, userWith("t1", "u1", []string{"s1"}, 5))
	ctx := actorCtx("t1", "u1")

	// Directly to available from offline must fail.
	_, err := svc.SetStatus(ctx, "", entity.StatusAvailable)
	if apperror.From(err).Code != apperror.CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}

	// online → available succeeds.
	if _, err := svc.SetStatus(ctx, "", entity.StatusOnline); err != nil {
		t.Fatalf("online: %v", err)
	}
	if _, err := svc.SetStatus(ctx, "", entity.StatusAvailable); err != nil {
		t.Errorf("available after online: %v", err)
	}
}

func TestSetStatus_AvailableRequiresSectors(t *testing.T) {
	svc, _, _ := newPresenceService(0, userWith("t1", "u1", nil, 5))
	ctx := actorCtx("t1", "u1")
	_ = mustOnline(t, svc, ctx)

	_, err := svc.SetStatus(ctx, "", entity.StatusAvailable)
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation_error without sectors, got %v", err)
	}
}

func TestSetStatus_CannotChangeOthersWithoutPermission(t *testing.T) {
	users := map[string]*iamentity.User{
		"u1": {ID: "u1", TenantID: "t1"},
		"u2": {ID: "u2", TenantID: "t1"},
	}
	svc, _, _ := newPresenceService(0, users)

	// u1 has no user.manage → cannot set u2's status.
	ctx := actorCtx("t1", "u1")
	if _, err := svc.SetStatus(ctx, "u2", entity.StatusOnline); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expected forbidden, got %v", err)
	}

	// Supervisor with user.manage can.
	sup := actorCtx("t1", "u1", authz.UserManage)
	if _, err := svc.SetStatus(sup, "u2", entity.StatusOnline); err != nil {
		t.Errorf("supervisor setting other status: %v", err)
	}
}

func TestSetStatus_InvalidStatus(t *testing.T) {
	svc, _, _ := newPresenceService(0, userWith("t1", "u1", []string{"s1"}, 5))
	ctx := actorCtx("t1", "u1")
	if _, err := svc.SetStatus(ctx, "", entity.Status("teleporting")); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation_error, got %v", err)
	}
}

func TestList_RecomputesLoad(t *testing.T) {
	svc, _, _ := newPresenceService(4, userWith("t1", "u1", []string{"s1"}, 10))
	ctx := actorCtx("t1", "u1")
	if _, err := svc.SetStatus(ctx, "", entity.StatusOnline); err != nil {
		t.Fatalf("seed: %v", err)
	}
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].CurrentLoad != 4 {
		t.Errorf("expected 1 agent with load 4, got %+v", list)
	}
}

func mustOnline(t *testing.T, svc *Service, ctx context.Context) *entity.AgentPresence {
	t.Helper()
	p, err := svc.SetStatus(ctx, "", entity.StatusOnline)
	if err != nil {
		t.Fatalf("online: %v", err)
	}
	return p
}
