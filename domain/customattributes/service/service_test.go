package service

import (
	"context"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── fake repo ────────────────────────────────────────────────────────────────

type fakeRepo struct {
	items map[string]*entity.Definition // id -> def
}

func newFakeRepo() *fakeRepo { return &fakeRepo{items: map[string]*entity.Definition{}} }

func (r *fakeRepo) Create(_ context.Context, d *entity.Definition) error {
	cp := *d
	r.items[d.ID] = &cp
	return nil
}
func (r *fakeRepo) Update(_ context.Context, d *entity.Definition) error {
	cp := *d
	r.items[d.ID] = &cp
	return nil
}
func (r *fakeRepo) Delete(_ context.Context, id string) error { delete(r.items, id); return nil }
func (r *fakeRepo) FindByID(_ context.Context, id string) (*entity.Definition, error) {
	if d, ok := r.items[id]; ok {
		return d, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeRepo) FindByKey(_ context.Context, appliesTo entity.AppliesTo, key string) (*entity.Definition, error) {
	for _, d := range r.items {
		if d.AppliesTo == appliesTo && d.Key == key {
			return d, nil
		}
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeRepo) List(_ context.Context, appliesTo entity.AppliesTo, _ shared.PageRequest) ([]*entity.Definition, error) {
	return r.ListAllByAppliesTo(context.Background(), appliesTo)
}
func (r *fakeRepo) ListAllByAppliesTo(_ context.Context, appliesTo entity.AppliesTo) ([]*entity.Definition, error) {
	var out []*entity.Definition
	for _, d := range r.items {
		if !appliesTo.Valid() || d.AppliesTo == appliesTo {
			out = append(out, d)
		}
	}
	return out, nil
}

func ctxT() context.Context { return shared.WithTenant(context.Background(), "t1") }

func mustCreate(t *testing.T, svc *Service, cmd contracts.CreateDefinition) *entity.Definition {
	t.Helper()
	d, err := svc.Create(ctxT(), cmd)
	if err != nil {
		t.Fatalf("create %q: %v", cmd.Key, err)
	}
	return d
}

// ── definition CRUD ──────────────────────────────────────────────────────────

func TestCreate_ValidatesShape(t *testing.T) {
	svc := New(newFakeRepo(), nil)
	cases := map[string]contracts.CreateDefinition{
		"missing key":       {Label: "L", Type: entity.TypeText, AppliesTo: entity.AppliesToContact},
		"missing label":     {Key: "k", Type: entity.TypeText, AppliesTo: entity.AppliesToContact},
		"bad type":          {Key: "k", Label: "L", Type: "carrier", AppliesTo: entity.AppliesToContact},
		"bad applies_to":    {Key: "k", Label: "L", Type: entity.TypeText, AppliesTo: "both"},
		"list without opts": {Key: "k", Label: "L", Type: entity.TypeList, AppliesTo: entity.AppliesToContact},
		"opts on non-list":  {Key: "k", Label: "L", Type: entity.TypeText, AppliesTo: entity.AppliesToContact, Options: []string{"a"}},
		"regex on non-text": {Key: "k", Label: "L", Type: entity.TypeNumber, AppliesTo: entity.AppliesToContact, Regex: "x"},
		"invalid regex":     {Key: "k", Label: "L", Type: entity.TypeText, AppliesTo: entity.AppliesToContact, Regex: "("},
	}
	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Create(ctxT(), cmd); apperror.From(err).Code != apperror.CodeValidation {
				t.Errorf("expected validation_error, got %v", err)
			}
		})
	}
}

func TestCreate_UniquePerScope(t *testing.T) {
	svc := New(newFakeRepo(), nil)
	base := contracts.CreateDefinition{Key: "plano", Label: "Plano", Type: entity.TypeText, AppliesTo: entity.AppliesToContact}
	mustCreate(t, svc, base)
	// Same key+scope → conflict.
	if _, err := svc.Create(ctxT(), base); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("expected conflict for duplicate key in same scope, got %v", err)
	}
	// Same key, OTHER scope → allowed.
	other := base
	other.AppliesTo = entity.AppliesToConversation
	if _, err := svc.Create(ctxT(), other); err != nil {
		t.Errorf("same key in a different scope must be allowed, got %v", err)
	}
}

func TestUpdate_KeyAndTypeImmutable_LabelEditable(t *testing.T) {
	svc := New(newFakeRepo(), nil)
	d := mustCreate(t, svc, contracts.CreateDefinition{Key: "plano", Label: "Plano", Type: entity.TypeText, AppliesTo: entity.AppliesToContact})
	newLabel := "Plano contratado"
	updated, err := svc.Update(ctxT(), d.ID, contracts.UpdateDefinition{Label: &newLabel})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Label != newLabel || updated.Key != "plano" || updated.Type != entity.TypeText {
		t.Errorf("label must change while key/type stay: %+v", updated)
	}
}

// ── value validation ─────────────────────────────────────────────────────────

func TestValidateCustomAttributes(t *testing.T) {
	svc := New(newFakeRepo(), nil)
	mustCreate(t, svc, contracts.CreateDefinition{Key: "idade", Label: "Idade", Type: entity.TypeNumber, AppliesTo: entity.AppliesToContact})
	mustCreate(t, svc, contracts.CreateDefinition{Key: "ativo", Label: "Ativo", Type: entity.TypeBoolean, AppliesTo: entity.AppliesToContact})
	mustCreate(t, svc, contracts.CreateDefinition{Key: "nascimento", Label: "Nascimento", Type: entity.TypeDate, AppliesTo: entity.AppliesToContact})
	mustCreate(t, svc, contracts.CreateDefinition{Key: "plano", Label: "Plano", Type: entity.TypeList, AppliesTo: entity.AppliesToContact, Options: []string{"Pro", "Free"}})
	mustCreate(t, svc, contracts.CreateDefinition{Key: "cpf", Label: "CPF", Type: entity.TypeText, AppliesTo: entity.AppliesToContact, Regex: `^[0-9]+$`})

	ok := map[string]any{"idade": float64(30), "ativo": true, "nascimento": "1994-05-01", "plano": "Pro", "cpf": "123"}
	if err := svc.ValidateCustomAttributes(ctxT(), "contact", ok); err != nil {
		t.Fatalf("valid attributes rejected: %v", err)
	}

	bad := []map[string]any{
		{"idade": "thirty"},        // number expects numeric
		{"ativo": "yes"},           // boolean expects bool
		{"nascimento": "01/05/94"}, // date expects YYYY-MM-DD
		{"plano": "Enterprise"},    // not in options
		{"cpf": "12a"},             // regex mismatch
		{"unknown": "x"},           // no definition
	}
	for _, attrs := range bad {
		if err := svc.ValidateCustomAttributes(ctxT(), "contact", attrs); apperror.From(err).Code != apperror.CodeValidation {
			t.Errorf("expected validation_error for %v, got %v", attrs, err)
		}
	}

	// A definition is scoped: a contact attribute is unknown for the conversation scope.
	if err := svc.ValidateCustomAttributes(ctxT(), "conversation", map[string]any{"idade": float64(30)}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("contact-scoped key must be unknown for conversation scope, got %v", err)
	}

	// Empty map is always valid.
	if err := svc.ValidateCustomAttributes(ctxT(), "contact", nil); err != nil {
		t.Errorf("empty map must be valid, got %v", err)
	}
}
