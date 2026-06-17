package service

import (
	"context"
	"errors"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/groups/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/groups/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fakeRepo struct {
	upsertCalls   [][]contracts.UpsertGroup
	upsertChannel string
	upsertN       int
	lastAttendID  string
	lastAttend    bool
	lastJID       string
	listFilter    contracts.ListFilter
}

func (r *fakeRepo) UpsertBatch(_ context.Context, channelID string, groups []contracts.UpsertGroup) (int, error) {
	r.upsertChannel = channelID
	r.upsertCalls = append(r.upsertCalls, groups)
	if r.upsertN != 0 {
		return r.upsertN, nil
	}
	return len(groups), nil
}
func (r *fakeRepo) FindByID(context.Context, string) (*entity.Group, error) {
	return &entity.Group{ID: "g1"}, nil
}
func (r *fakeRepo) FindByJID(_ context.Context, jid string) (*entity.Group, error) {
	r.lastJID = jid
	return &entity.Group{ID: "g1", GroupJID: jid}, nil
}
func (r *fakeRepo) List(_ context.Context, f contracts.ListFilter, _ shared.PageRequest) ([]*entity.Group, error) {
	r.listFilter = f
	return []*entity.Group{{ID: "g1"}}, nil
}
func (r *fakeRepo) SetAttend(_ context.Context, id string, attend bool) (*entity.Group, error) {
	r.lastAttendID = id
	r.lastAttend = attend
	return &entity.Group{ID: id, Attend: attend}, nil
}

type fakeEmitter struct {
	calls  int
	event  string
	chanID string
	err    error
}

func (e *fakeEmitter) EmitToChannel(_ context.Context, _, channelID, event string, _ any) error {
	e.calls++
	e.event = event
	e.chanID = channelID
	return e.err
}

func tenantCtx() context.Context {
	return shared.WithTenant(context.Background(), "t1")
}

func TestUpsertBatchSkipsEmptyJIDAndDelegates(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, nil, nil)
	groups := []contracts.UpsertGroup{
		{GroupJID: "g1@g.us", Name: "A"},
		{GroupJID: "  ", Name: "no jid"},
		{GroupJID: "g2@g.us", Name: "B"},
	}
	n, err := svc.UpsertBatch(tenantCtx(), "ch1", groups)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Fatalf("want 2 upserted, got %d", n)
	}
	if repo.upsertChannel != "ch1" {
		t.Fatalf("want channel ch1, got %q", repo.upsertChannel)
	}
	if got := len(repo.upsertCalls[0]); got != 2 {
		t.Fatalf("want 2 clean groups passed to repo, got %d", got)
	}
}

func TestUpsertBatchRejectsEmpty(t *testing.T) {
	svc := New(&fakeRepo{}, nil, nil)
	if _, err := svc.UpsertBatch(tenantCtx(), "ch1", nil); apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("want validation error, got %v", err)
	}
}

func TestUpsertBatchRejectsTooLarge(t *testing.T) {
	svc := New(&fakeRepo{}, nil, nil)
	big := make([]contracts.UpsertGroup, maxSyncBatch+1)
	if _, err := svc.UpsertBatch(tenantCtx(), "ch1", big); apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("want validation error, got %v", err)
	}
}

func TestUpsertBatchRequiresTenant(t *testing.T) {
	svc := New(&fakeRepo{}, nil, nil)
	if _, err := svc.UpsertBatch(context.Background(), "ch1", []contracts.UpsertGroup{{GroupJID: "g1"}}); err == nil {
		t.Fatal("want tenant error, got nil")
	}
}

func TestSyncEmitsEvent(t *testing.T) {
	em := &fakeEmitter{}
	svc := New(&fakeRepo{}, em, nil)
	if err := svc.Sync(tenantCtx(), "ch1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if em.calls != 1 {
		t.Fatalf("want 1 emit, got %d", em.calls)
	}
	if em.event != eventGroupSyncRequested {
		t.Fatalf("want event %q, got %q", eventGroupSyncRequested, em.event)
	}
	if em.chanID != "ch1" {
		t.Fatalf("want channel ch1, got %q", em.chanID)
	}
}

func TestSyncWithoutEmitterFails(t *testing.T) {
	svc := New(&fakeRepo{}, nil, nil)
	if err := svc.Sync(tenantCtx(), "ch1"); apperror.From(err).Code != apperror.CodeIntegrationUnavailable {
		t.Fatalf("want integration error, got %v", err)
	}
}

func TestSyncPropagatesEmitterError(t *testing.T) {
	em := &fakeEmitter{err: errors.New("no managed webhook")}
	svc := New(&fakeRepo{}, em, nil)
	if err := svc.Sync(tenantCtx(), "ch1"); err == nil {
		t.Fatal("want error from emitter, got nil")
	}
}

func TestSyncRequiresChannelID(t *testing.T) {
	svc := New(&fakeRepo{}, &fakeEmitter{}, nil)
	if err := svc.Sync(tenantCtx(), "  "); apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("want validation error, got %v", err)
	}
}

func TestSetAttendDelegates(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, nil, nil)
	g, err := svc.SetAttend(tenantCtx(), "g1", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastAttendID != "g1" || repo.lastAttend != false {
		t.Fatalf("repo not called correctly: id=%q attend=%v", repo.lastAttendID, repo.lastAttend)
	}
	if g.Attend != false {
		t.Fatalf("want attend false, got %v", g.Attend)
	}
}

func TestListPassesFilter(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, nil, nil)
	want := true
	_, err := svc.List(tenantCtx(), contracts.ListFilter{Q: "vip", Attend: &want}, shared.PageRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.listFilter.Q != "vip" || repo.listFilter.Attend == nil || *repo.listFilter.Attend != true {
		t.Fatalf("filter not forwarded: %+v", repo.listFilter)
	}
}

func TestFindByJIDTrimsAndDelegates(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, nil, nil)
	g, err := svc.FindByJID(tenantCtx(), "  g1@g.us  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastJID != "g1@g.us" {
		t.Fatalf("want trimmed jid, got %q", repo.lastJID)
	}
	if g.GroupJID != "g1@g.us" {
		t.Fatalf("want group jid, got %q", g.GroupJID)
	}
}
