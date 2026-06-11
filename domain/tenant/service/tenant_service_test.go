package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/entity"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeTenantRepo struct {
	tenants map[string]*entity.Tenant
}

func (r *fakeTenantRepo) FindByID(_ context.Context, id string) (*entity.Tenant, error) {
	if t, ok := r.tenants[id]; ok {
		cp := *t
		return &cp, nil
	}
	return nil, apperror.NotFound("resource not found")
}
func (r *fakeTenantRepo) ListActive(_ context.Context) ([]*entity.Tenant, error) {
	var out []*entity.Tenant
	for _, t := range r.tenants {
		out = append(out, t)
	}
	return out, nil
}
func (r *fakeTenantRepo) Update(_ context.Context, t *entity.Tenant) error {
	if _, ok := r.tenants[t.ID]; !ok {
		return apperror.NotFound("resource not found")
	}
	cp := *t
	r.tenants[t.ID] = &cp
	return nil
}

func newService() *Service {
	repo := &fakeTenantRepo{tenants: map[string]*entity.Tenant{
		"t1": {ID: "t1", Name: "Acme", Status: entity.StatusActive},
	}}
	return New(repo, fixedClock{t: time.Unix(1700000000, 0).UTC()})
}

func TestCurrent_RequiresTenant(t *testing.T) {
	if _, err := newService().Current(context.Background()); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expected forbidden without tenant, got %v", err)
	}
}

func TestCurrent_ReturnsScopedTenant(t *testing.T) {
	ctx := shared.WithTenant(context.Background(), "t1")
	got, err := newService().Current(ctx)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if got.ID != "t1" || got.Name != "Acme" {
		t.Errorf("unexpected tenant: %+v", got)
	}
}

func TestUpdateSettings(t *testing.T) {
	ctx := shared.WithTenant(context.Background(), "t1")
	svc := newService()
	got, err := svc.UpdateSettings(ctx, "Acme Corp", map[string]any{"theme": "dark"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.Name != "Acme Corp" {
		t.Errorf("name not updated: %q", got.Name)
	}
	if got.Settings["theme"] != "dark" {
		t.Errorf("settings not updated: %+v", got.Settings)
	}
}
