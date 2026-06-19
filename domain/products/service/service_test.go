package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/products/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/products/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fakeRepo struct{ byID map[string]*entity.Product }

func newRepo() *fakeRepo { return &fakeRepo{byID: map[string]*entity.Product{}} }

func (r *fakeRepo) Create(ctx context.Context, p *entity.Product) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	cp := *p
	r.byID[p.ID] = &cp
	return nil
}
func (r *fakeRepo) Update(ctx context.Context, p *entity.Product) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if _, ok := r.byID[p.ID]; !ok {
		return apperror.NotFound("nf")
	}
	cp := *p
	r.byID[p.ID] = &cp
	return nil
}
func (r *fakeRepo) FindByID(ctx context.Context, id string) (*entity.Product, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	if p, ok := r.byID[id]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeRepo) List(ctx context.Context, f contracts.ListFilter, _ shared.PageRequest) ([]*entity.Product, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	var out []*entity.Product
	for _, p := range r.byID {
		if f.Active != nil && p.Active != *f.Active {
			continue
		}
		cp := *p
		out = append(out, &cp)
	}
	return out, nil
}

type fakeGate struct{ on bool }

func (g fakeGate) ProductsEnabled(context.Context) (bool, error) { return g.on, nil }

func ctx() context.Context { return shared.WithTenant(context.Background(), "t1") }

func newSvc(on bool) (*Service, *fakeRepo) {
	repo := newRepo()
	svc := New(repo, nil)
	svc.SetModuleGate(fakeGate{on: on})
	return svc, repo
}

func ptrBool(b bool) *bool        { return &b }
func ptrFloat(f float64) *float64 { return &f }

func TestCreate_DefaultsAndValidations(t *testing.T) {
	svc, _ := newSvc(true)

	p, err := svc.Create(ctx(), contracts.CreateProduct{Name: "Plano 500MB", Price: 99.9})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Currency != "BRL" || !p.Active {
		t.Errorf("defaults wrong: currency=%q active=%v", p.Currency, p.Active)
	}
	if _, err := svc.Create(ctx(), contracts.CreateProduct{Name: "  "}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("empty name must be validation, got %v", err)
	}
	if _, err := svc.Create(ctx(), contracts.CreateProduct{Name: "x", Price: -1}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("negative price must be validation, got %v", err)
	}
}

func TestCreate_RejectedWhenModuleDisabled(t *testing.T) {
	svc, _ := newSvc(false)
	if _, err := svc.Create(ctx(), contracts.CreateProduct{Name: "x", Price: 1}); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("disabled module must be a conflict, got %v", err)
	}
}

func TestUpdate_EditsAndDeactivates(t *testing.T) {
	svc, _ := newSvc(true)
	p, _ := svc.Create(ctx(), contracts.CreateProduct{Name: "Plano", Price: 50})

	upd, err := svc.Update(ctx(), p.ID, contracts.UpdateProduct{Price: ptrFloat(75), Active: ptrBool(false)})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if upd.Price != 75 || upd.Active {
		t.Errorf("update/deactivate wrong: %+v", upd)
	}
}

func TestList_EmptyWhenDisabledAndFilters(t *testing.T) {
	svc, _ := newSvc(true)
	a, _ := svc.Create(ctx(), contracts.CreateProduct{Name: "Ativo", Price: 1})
	b, _ := svc.Create(ctx(), contracts.CreateProduct{Name: "Inativo", Price: 1})
	_, _ = svc.Update(ctx(), b.ID, contracts.UpdateProduct{Active: ptrBool(false)})

	active, err := svc.List(ctx(), contracts.ListFilter{Active: ptrBool(true)}, shared.PageRequest{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(active) != 1 || active[0].ID != a.ID {
		t.Errorf("active filter wrong: %+v", active)
	}

	// Disabled module → empty.
	off, _ := newSvc(false)
	if items, _ := off.List(ctx(), contracts.ListFilter{}, shared.PageRequest{Limit: 10}); len(items) != 0 {
		t.Errorf("disabled must return empty, got %v", items)
	}
}
