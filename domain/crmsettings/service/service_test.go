package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// fakeRepo is an in-memory CRMSettingsRepository keyed by tenant.
type fakeRepo struct {
	byTenant map[string]*entity.CRMSettings
}

func newRepo() *fakeRepo { return &fakeRepo{byTenant: map[string]*entity.CRMSettings{}} }

func (r *fakeRepo) Get(ctx context.Context) (*entity.CRMSettings, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	s, ok := r.byTenant[tenantID]
	if !ok {
		return nil, apperror.NotFound("no settings")
	}
	cp := *s
	return &cp, nil
}

func (r *fakeRepo) Upsert(ctx context.Context, s *entity.CRMSettings) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	cp := *s
	cp.TenantID = tenantID
	r.byTenant[tenantID] = &cp
	return nil
}

func tenantCtx(id string) context.Context { return shared.WithTenant(context.Background(), id) }

func ptr(b bool) *bool { return &b }

func TestGet_DefaultsWhenNeverConfigured(t *testing.T) {
	svc := New(newRepo(), nil)
	s, err := svc.Get(tenantCtx("t1"))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	// Conservative defaults: timeline on, tasks/products off.
	if !s.TimelineEnabled || s.TasksEnabled || s.ProductsEnabled {
		t.Errorf("wrong defaults: %+v", s)
	}
	if s.TenantID != "t1" {
		t.Errorf("defaults must carry the tenant id, got %q", s.TenantID)
	}
}

func TestGet_RequiresTenant(t *testing.T) {
	svc := New(newRepo(), nil)
	if _, err := svc.Get(context.Background()); err == nil {
		t.Error("missing tenant must error")
	}
}

func TestUpdate_TogglesAndPersists(t *testing.T) {
	repo := newRepo()
	svc := New(repo, nil)
	ctx := tenantCtx("t1")

	// Enable tasks only; timeline stays on (default), products stays off.
	s, err := svc.Update(ctx, contracts.UpdateCRMSettings{TasksEnabled: ptr(true)})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !s.TasksEnabled || !s.TimelineEnabled || s.ProductsEnabled {
		t.Errorf("partial toggle wrong: %+v", s)
	}
	if s.UpdatedAt.IsZero() {
		t.Error("updated_at must be set on update")
	}

	// A second PATCH leaves untouched fields as they were (tasks stays on).
	s2, err := svc.Update(ctx, contracts.UpdateCRMSettings{ProductsEnabled: ptr(true)})
	if err != nil {
		t.Fatalf("update 2: %v", err)
	}
	if !s2.TasksEnabled || !s2.ProductsEnabled || !s2.TimelineEnabled {
		t.Errorf("PATCH must preserve unspecified fields: %+v", s2)
	}

	// Persisted: a fresh Get returns the stored doc, not the defaults.
	got, _ := svc.Get(ctx)
	if !got.TasksEnabled || !got.ProductsEnabled {
		t.Errorf("not persisted: %+v", got)
	}
}

func TestUpdate_CanDisableTimeline(t *testing.T) {
	svc := New(newRepo(), nil)
	s, err := svc.Update(tenantCtx("t1"), contracts.UpdateCRMSettings{TimelineEnabled: ptr(false)})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if s.TimelineEnabled {
		t.Error("timeline must be toggleable off")
	}
}

func TestUpdate_TenantScoped(t *testing.T) {
	repo := newRepo()
	svc := New(repo, nil)
	if _, err := svc.Update(tenantCtx("t1"), contracts.UpdateCRMSettings{ProductsEnabled: ptr(true)}); err != nil {
		t.Fatalf("update t1: %v", err)
	}
	// Another tenant is unaffected — it still sees the conservative defaults.
	other, err := svc.Get(tenantCtx("t2"))
	if err != nil {
		t.Fatalf("get t2: %v", err)
	}
	if other.ProductsEnabled {
		t.Errorf("tenant isolation broken: t2 saw t1's settings: %+v", other)
	}
}

func TestIsModuleEnabled(t *testing.T) {
	svc := New(newRepo(), nil)
	ctx := tenantCtx("t1")

	// Defaults: timeline on, the rest off.
	if on, _ := svc.IsModuleEnabled(ctx, entity.ModuleTimeline); !on {
		t.Error("timeline must be enabled by default")
	}
	if on, _ := svc.IsModuleEnabled(ctx, entity.ModuleTasks); on {
		t.Error("tasks must be disabled by default")
	}

	// After enabling products, the checkpoint reflects it.
	if _, err := svc.Update(ctx, contracts.UpdateCRMSettings{ProductsEnabled: ptr(true)}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if on, _ := svc.IsModuleEnabled(ctx, entity.ModuleProducts); !on {
		t.Error("products must be enabled after the toggle")
	}
	// An unknown module is never enabled.
	if on, _ := svc.IsModuleEnabled(ctx, entity.Module("ghost")); on {
		t.Error("unknown module must be disabled")
	}
}
