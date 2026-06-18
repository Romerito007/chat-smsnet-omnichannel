package service

import (
	"context"
	"errors"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fakeRepo struct {
	byID map[string]*entity.Pipeline
}

func newFakeRepo() *fakeRepo { return &fakeRepo{byID: map[string]*entity.Pipeline{}} }

func (r *fakeRepo) Create(_ context.Context, p *entity.Pipeline) error {
	cp := *p
	r.byID[p.ID] = &cp
	return nil
}
func (r *fakeRepo) Update(_ context.Context, p *entity.Pipeline) error {
	if _, ok := r.byID[p.ID]; !ok {
		return apperror.NotFound("nf")
	}
	cp := *p
	r.byID[p.ID] = &cp
	return nil
}
func (r *fakeRepo) Delete(_ context.Context, id string) error {
	if _, ok := r.byID[id]; !ok {
		return apperror.NotFound("nf")
	}
	delete(r.byID, id)
	return nil
}
func (r *fakeRepo) FindByID(_ context.Context, id string) (*entity.Pipeline, error) {
	if p, ok := r.byID[id]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeRepo) List(_ context.Context) ([]*entity.Pipeline, error) {
	out := make([]*entity.Pipeline, 0, len(r.byID))
	for _, p := range r.byID {
		cp := *p
		out = append(out, &cp)
	}
	return out, nil
}
func (r *fakeRepo) CountByTenant(_ context.Context) (int, error) { return len(r.byID), nil }
func (r *fakeRepo) ClearDefault(_ context.Context, keepID string) error {
	for id, p := range r.byID {
		if id != keepID {
			p.IsDefault = false
		}
	}
	return nil
}

func tenantCtx() context.Context { return shared.WithTenant(context.Background(), "t1") }

func defaultStages() []contracts.StageInput {
	return []contracts.StageInput{
		{Name: "Novo lead", Order: 0},
		{Name: "Proposta", Order: 1},
		{Name: "Ganho", Order: 2, IsWon: true},
		{Name: "Perdido", Order: 3, IsLost: true},
	}
}

func TestCreate_FirstBecomesDefault(t *testing.T) {
	svc := New(newFakeRepo(), nil)
	ctx := tenantCtx()

	p1, err := svc.Create(ctx, contracts.CreatePipeline{Name: "Vendas", Stages: defaultStages()})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !p1.IsDefault {
		t.Errorf("the first pipeline must become the tenant default")
	}
	if len(p1.Stages) != 4 || p1.Stages[0].ID == "" {
		t.Errorf("stages must be stored with ids: %+v", p1.Stages)
	}
	// A second pipeline is NOT default automatically.
	p2, err := svc.Create(ctx, contracts.CreatePipeline{Name: "Parcerias", Stages: defaultStages()})
	if err != nil {
		t.Fatalf("create 2: %v", err)
	}
	if p2.IsDefault {
		t.Errorf("the second pipeline must not be default")
	}
}

func TestCreate_RequiresTenantAndName(t *testing.T) {
	svc := New(newFakeRepo(), nil)
	if _, err := svc.Create(context.Background(), contracts.CreatePipeline{Name: "X"}); err == nil {
		t.Error("missing tenant must error")
	}
	if _, err := svc.Create(tenantCtx(), contracts.CreatePipeline{Name: "  "}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("empty name must be a validation error, got %v", err)
	}
}

func TestCreate_RejectsMultipleTerminals(t *testing.T) {
	svc := New(newFakeRepo(), nil)
	_, err := svc.Create(tenantCtx(), contracts.CreatePipeline{Name: "X", Stages: []contracts.StageInput{
		{Name: "G1", IsWon: true}, {Name: "G2", IsWon: true},
	}})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("two won stages must be rejected, got %v", err)
	}
}

func TestUpdate_SetDefaultClearsOthers(t *testing.T) {
	repo := newFakeRepo()
	svc := New(repo, nil)
	ctx := tenantCtx()
	p1, _ := svc.Create(ctx, contracts.CreatePipeline{Name: "A", Stages: defaultStages()}) // default
	p2, _ := svc.Create(ctx, contracts.CreatePipeline{Name: "B", Stages: defaultStages()})

	yes := true
	updated, err := svc.Update(ctx, p2.ID, contracts.UpdatePipeline{IsDefault: &yes})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !updated.IsDefault {
		t.Errorf("p2 must become default")
	}
	// p1 lost the default flag.
	got1, _ := repo.FindByID(ctx, p1.ID)
	if got1.IsDefault {
		t.Errorf("only one default per tenant: p1 must have been cleared")
	}
}

func TestUpdate_Rename(t *testing.T) {
	svc := New(newFakeRepo(), nil)
	ctx := tenantCtx()
	p, _ := svc.Create(ctx, contracts.CreatePipeline{Name: "Old", Stages: defaultStages()})
	name := "New"
	got, err := svc.Update(ctx, p.ID, contracts.UpdatePipeline{Name: &name})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if got.Name != "New" {
		t.Errorf("rename failed: %q", got.Name)
	}
}

func TestStages_AddUpdateDelete(t *testing.T) {
	svc := New(newFakeRepo(), nil)
	ctx := tenantCtx()
	p, _ := svc.Create(ctx, contracts.CreatePipeline{Name: "X", Stages: []contracts.StageInput{{Name: "Novo", Order: 0}}})

	// Add.
	p, err := svc.AddStage(ctx, p.ID, contracts.AddStage{Name: "Proposta", Order: 1})
	if err != nil || len(p.Stages) != 2 {
		t.Fatalf("add stage: %v (%+v)", err, p.Stages)
	}
	stageID := p.Stages[1].ID

	// Update.
	name := "Proposta enviada"
	p, err = svc.UpdateStage(ctx, p.ID, stageID, contracts.UpdateStage{Name: &name})
	if err != nil {
		t.Fatalf("update stage: %v", err)
	}
	if p.Stages[p.StageIndex(stageID)].Name != "Proposta enviada" {
		t.Errorf("stage not renamed: %+v", p.Stages)
	}

	// Delete.
	p, err = svc.DeleteStage(ctx, p.ID, stageID)
	if err != nil || len(p.Stages) != 1 {
		t.Fatalf("delete stage: %v (%+v)", err, p.Stages)
	}
}

func TestReorderStages(t *testing.T) {
	svc := New(newFakeRepo(), nil)
	ctx := tenantCtx()
	p, _ := svc.Create(ctx, contracts.CreatePipeline{Name: "X", Stages: []contracts.StageInput{
		{Name: "A", Order: 0}, {Name: "B", Order: 1}, {Name: "C", Order: 2},
	}})
	ids := []string{p.Stages[2].ID, p.Stages[0].ID, p.Stages[1].ID} // C,A,B

	got, err := svc.ReorderStages(ctx, p.ID, contracts.ReorderStages{StageIDs: ids})
	if err != nil {
		t.Fatalf("reorder: %v", err)
	}
	// Returned sorted by new order → C, A, B.
	if got.Stages[0].Name != "C" || got.Stages[1].Name != "A" || got.Stages[2].Name != "B" {
		t.Errorf("reorder did not apply: %+v", got.Stages)
	}
}

func TestReorderStages_RejectsIncompleteList(t *testing.T) {
	svc := New(newFakeRepo(), nil)
	ctx := tenantCtx()
	p, _ := svc.Create(ctx, contracts.CreatePipeline{Name: "X", Stages: []contracts.StageInput{
		{Name: "A", Order: 0}, {Name: "B", Order: 1},
	}})
	if _, err := svc.ReorderStages(ctx, p.ID, contracts.ReorderStages{StageIDs: []string{p.Stages[0].ID}}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("a partial reorder must be rejected, got %v", err)
	}
}

// fakeDealChecker reports a stage as holding deals, to exercise the delete guard.
type fakeDealChecker struct {
	hasDeals bool
	err      error
}

func (f fakeDealChecker) StageHasDeals(context.Context, string, string) (bool, error) {
	return f.hasDeals, f.err
}

func TestDeleteStage_BlockedWhenHasDeals(t *testing.T) {
	svc := New(newFakeRepo(), nil)
	svc.SetDealChecker(fakeDealChecker{hasDeals: true})
	ctx := tenantCtx()
	p, _ := svc.Create(ctx, contracts.CreatePipeline{Name: "X", Stages: []contracts.StageInput{{Name: "A", Order: 0}, {Name: "B", Order: 1}}})

	if _, err := svc.DeleteStage(ctx, p.ID, p.Stages[0].ID); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("deleting a stage with deals must conflict, got %v", err)
	}
	// A checker error propagates.
	svc.SetDealChecker(fakeDealChecker{err: errors.New("boom")})
	if _, err := svc.DeleteStage(ctx, p.ID, p.Stages[0].ID); err == nil {
		t.Error("a deal-checker error must propagate")
	}
}

func TestList_TenantScopedAndSorted(t *testing.T) {
	svc := New(newFakeRepo(), nil)
	if _, err := svc.List(context.Background()); err == nil {
		t.Error("list without tenant must error")
	}
	ctx := tenantCtx()
	_, _ = svc.Create(ctx, contracts.CreatePipeline{Name: "X", Stages: []contracts.StageInput{
		{Name: "B", Order: 1}, {Name: "A", Order: 0},
	}})
	items, err := svc.List(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %v n=%d", err, len(items))
	}
	if items[0].Stages[0].Name != "A" {
		t.Errorf("stages must be sorted by order: %+v", items[0].Stages)
	}
}
