package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeSectorRepo struct {
	sectors map[string]*entity.Sector
}

func newFakeSectorRepo() *fakeSectorRepo {
	return &fakeSectorRepo{sectors: map[string]*entity.Sector{}}
}

func (r *fakeSectorRepo) Create(_ context.Context, s *entity.Sector) error {
	cp := *s
	r.sectors[s.ID] = &cp
	return nil
}
func (r *fakeSectorRepo) Update(_ context.Context, s *entity.Sector) error {
	if _, ok := r.sectors[s.ID]; !ok {
		return apperror.NotFound("resource not found")
	}
	cp := *s
	r.sectors[s.ID] = &cp
	return nil
}
func (r *fakeSectorRepo) Delete(_ context.Context, id string) error {
	delete(r.sectors, id)
	return nil
}
func (r *fakeSectorRepo) FindByID(ctx context.Context, id string) (*entity.Sector, error) {
	tenant, _ := shared.TenantFrom(ctx)
	if s, ok := r.sectors[id]; ok && s.TenantID == tenant {
		cp := *s
		return &cp, nil
	}
	return nil, apperror.NotFound("resource not found")
}
func (r *fakeSectorRepo) List(ctx context.Context, _ shared.PageRequest) ([]*entity.Sector, error) {
	tenant, _ := shared.TenantFrom(ctx)
	var out []*entity.Sector
	for _, s := range r.sectors {
		if s.TenantID == tenant {
			cp := *s
			out = append(out, &cp)
		}
	}
	return out, nil
}

func tenantCtx(tenant string) context.Context {
	return shared.WithTenant(context.Background(), tenant)
}

func newSectorService() (*Service, *fakeSectorRepo) {
	repo := newFakeSectorRepo()
	return New(repo, fixedClock{t: time.Unix(1700000000, 0).UTC()}), repo
}

func TestCreateSector_DefaultsEnabledTrue(t *testing.T) {
	svc, _ := newSectorService()
	s, err := svc.Create(tenantCtx("t1"), contracts.CreateSector{Name: "Support"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !s.Enabled {
		t.Error("expected enabled to default to true")
	}
	if s.TenantID != "t1" {
		t.Errorf("tenant = %q", s.TenantID)
	}
}

func TestCreateSector_RequiresName(t *testing.T) {
	svc, _ := newSectorService()
	if _, err := svc.Create(tenantCtx("t1"), contracts.CreateSector{Name: "  "}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation_error, got %v", err)
	}
}

func TestSector_TenantIsolation(t *testing.T) {
	svc, _ := newSectorService()
	s, err := svc.Create(tenantCtx("t1"), contracts.CreateSector{Name: "Sales"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Get(tenantCtx("t2"), s.ID); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("expected not_found cross-tenant, got %v", err)
	}
}

func TestUpdateSector_TogglesEnabled(t *testing.T) {
	svc, _ := newSectorService()
	s, _ := svc.Create(tenantCtx("t1"), contracts.CreateSector{Name: "Sales"})
	off := false
	updated, err := svc.Update(tenantCtx("t1"), s.ID, contracts.UpdateSector{Enabled: &off})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Enabled {
		t.Error("expected enabled=false after update")
	}
}
