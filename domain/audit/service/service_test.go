package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/audit/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/audit/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// fakeRepo records created logs and serves them back, capturing the List filter.
type fakeRepo struct {
	created   []*entity.AuditLog
	gotFilter repository.Filter
}

func (r *fakeRepo) Create(_ context.Context, l *entity.AuditLog) error {
	r.created = append(r.created, l)
	return nil
}
func (r *fakeRepo) List(_ context.Context, f repository.Filter, _ shared.PageRequest) ([]*entity.AuditLog, error) {
	r.gotFilter = f
	return r.created, nil
}

func userCtx(tenant, user string) context.Context {
	ctx := shared.WithTenant(context.Background(), tenant)
	ctx = shared.WithAuditMeta(ctx, "10.0.0.9", "agent/1.0")
	return authz.WithAuthContext(ctx, authz.NewAuthContext(tenant, user, authz.AllPermissions(), nil, authz.ScopeAll))
}

func TestRecord_FillsActorIPUserAgentAndTenantFromContext(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, fixedClock{t: time.Unix(1700000000, 0).UTC()})

	err := svc.Record(userCtx("t1", "owner-1"), shared.AuditEntry{
		Action: "user.created", ResourceType: "user", ResourceID: "u2",
	})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected one audit log, got %d", len(repo.created))
	}
	l := repo.created[0]
	if l.TenantID != "t1" || l.ActorID != "owner-1" || l.ActorType != shared.ActorTypeUser {
		t.Errorf("actor/tenant not filled: %+v", l)
	}
	if l.IP != "10.0.0.9" || l.UserAgent != "agent/1.0" {
		t.Errorf("ip/user_agent not filled: %+v", l)
	}
	if l.Action != "user.created" || l.ResourceID != "u2" || l.ID == "" || l.CreatedAt.IsZero() {
		t.Errorf("unexpected log: %+v", l)
	}
}

func TestRecord_SystemActorType(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, nil)
	ctx := shared.WithTenant(context.Background(), "t1")
	ctx = authz.WithAuthContext(ctx, authz.SystemActor("t1")) // UserID == "system"

	if err := svc.Record(ctx, shared.AuditEntry{Action: "privacy.retention.applied"}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if got := repo.created[0].ActorType; got != shared.ActorTypeSystem {
		t.Errorf("actor_type = %q, want system", got)
	}
}

func TestRecord_ExplicitTenantWins(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, nil)
	// No tenant on the context, but the entry carries one (e.g. from a job).
	err := svc.Record(context.Background(), shared.AuditEntry{TenantID: "tX", Action: "auth.login"})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if repo.created[0].TenantID != "tX" {
		t.Errorf("explicit tenant must win, got %q", repo.created[0].TenantID)
	}
}

func TestRecord_NoTenantIsError(t *testing.T) {
	svc := NewService(&fakeRepo{}, nil)
	err := svc.Record(context.Background(), shared.AuditEntry{Action: "x"})
	if apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expected forbidden without tenant, got %v", err)
	}
}

func TestList_RequiresTenantAndPassesFilter(t *testing.T) {
	repo := &fakeRepo{created: []*entity.AuditLog{{ID: "a1", Action: "user.created"}}}
	svc := NewService(repo, nil)

	items, err := svc.List(userCtx("t1", "owner-1"), repository.Filter{Action: "user.", ResourceID: "u2"}, shared.PageRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
	if repo.gotFilter.Action != "user." || repo.gotFilter.ResourceID != "u2" {
		t.Errorf("filter not passed through: %+v", repo.gotFilter)
	}

	if _, err := svc.List(context.Background(), repository.Filter{}, shared.PageRequest{}); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expected forbidden without tenant, got %v", err)
	}
}
