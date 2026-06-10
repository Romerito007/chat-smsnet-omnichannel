package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func ctxT() context.Context { return shared.WithTenant(context.Background(), "t1") }

// ── tag repo fake ────────────────────────────────────────────────────────────

type fakeTagRepo struct{ items map[string]*entity.Tag }

func newTagRepo() *fakeTagRepo { return &fakeTagRepo{items: map[string]*entity.Tag{}} }
func (r *fakeTagRepo) Create(_ context.Context, t *entity.Tag) error {
	cp := *t
	r.items[t.ID] = &cp
	return nil
}
func (r *fakeTagRepo) Update(_ context.Context, t *entity.Tag) error {
	if _, ok := r.items[t.ID]; !ok {
		return apperror.NotFound("nf")
	}
	cp := *t
	r.items[t.ID] = &cp
	return nil
}
func (r *fakeTagRepo) Delete(_ context.Context, id string) error { delete(r.items, id); return nil }
func (r *fakeTagRepo) FindByID(_ context.Context, id string) (*entity.Tag, error) {
	if t, ok := r.items[id]; ok {
		return t, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeTagRepo) List(context.Context, shared.PageRequest) ([]*entity.Tag, error) {
	out := make([]*entity.Tag, 0, len(r.items))
	for _, t := range r.items {
		out = append(out, t)
	}
	return out, nil
}
func (r *fakeTagRepo) FindByIDs(_ context.Context, ids []string) ([]*entity.Tag, error) {
	var out []*entity.Tag
	for _, id := range ids {
		if t, ok := r.items[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

// ── canned repo fake ─────────────────────────────────────────────────────────

type fakeCannedRepo struct {
	items map[string]*entity.CannedResponse
}

func newCannedRepo() *fakeCannedRepo {
	return &fakeCannedRepo{items: map[string]*entity.CannedResponse{}}
}
func (r *fakeCannedRepo) Create(_ context.Context, c *entity.CannedResponse) error {
	cp := *c
	r.items[c.ID] = &cp
	return nil
}
func (r *fakeCannedRepo) Update(_ context.Context, c *entity.CannedResponse) error {
	cp := *c
	r.items[c.ID] = &cp
	return nil
}
func (r *fakeCannedRepo) Delete(_ context.Context, id string) error { delete(r.items, id); return nil }
func (r *fakeCannedRepo) FindByID(_ context.Context, id string) (*entity.CannedResponse, error) {
	if c, ok := r.items[id]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeCannedRepo) FindByShortcut(_ context.Context, shortcut string) (*entity.CannedResponse, error) {
	for _, c := range r.items {
		if c.Shortcut == shortcut {
			return c, nil
		}
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeCannedRepo) List(context.Context, shared.PageRequest) ([]*entity.CannedResponse, error) {
	return nil, nil
}

// ── close reason repo fake ───────────────────────────────────────────────────

type fakeCloseRepo struct {
	items map[string]*entity.CloseReason
}

func newCloseRepo() *fakeCloseRepo { return &fakeCloseRepo{items: map[string]*entity.CloseReason{}} }
func (r *fakeCloseRepo) Create(_ context.Context, c *entity.CloseReason) error {
	cp := *c
	r.items[c.ID] = &cp
	return nil
}
func (r *fakeCloseRepo) Update(_ context.Context, c *entity.CloseReason) error {
	cp := *c
	r.items[c.ID] = &cp
	return nil
}
func (r *fakeCloseRepo) Delete(_ context.Context, id string) error { delete(r.items, id); return nil }
func (r *fakeCloseRepo) FindByID(_ context.Context, id string) (*entity.CloseReason, error) {
	if c, ok := r.items[id]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeCloseRepo) List(context.Context, shared.PageRequest) ([]*entity.CloseReason, error) {
	return nil, nil
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestTag_CRUDAndValidate(t *testing.T) {
	svc := NewTagService(newTagRepo(), fixedClock{t: time.Unix(1700000000, 0).UTC()})

	tag, err := svc.Create(ctxT(), contracts.CreateTag{Name: "Urgent", Color: "#f00"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !tag.Enabled {
		t.Errorf("expected enabled by default")
	}

	// ValidateTags accepts a known+enabled tag.
	if err := svc.ValidateTags(ctxT(), []string{tag.ID}); err != nil {
		t.Errorf("validate known tag: %v", err)
	}
	// Unknown tag is rejected.
	if err := svc.ValidateTags(ctxT(), []string{"ghost"}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation error for unknown tag, got %v", err)
	}
	// Disabled tag is rejected.
	off := false
	if _, err := svc.Update(ctxT(), tag.ID, contracts.UpdateTag{Enabled: &off}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if err := svc.ValidateTags(ctxT(), []string{tag.ID}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation error for disabled tag, got %v", err)
	}
}

func TestTag_CreateRequiresName(t *testing.T) {
	svc := NewTagService(newTagRepo(), fixedClock{})
	if _, err := svc.Create(ctxT(), contracts.CreateTag{}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation error for missing name, got %v", err)
	}
}

func TestCanned_ResolveReturnsBodyAndRespectsSectors(t *testing.T) {
	svc := NewCannedResponseService(newCannedRepo(), fixedClock{t: time.Unix(1700000000, 0).UTC()})

	// A sector-restricted canned response.
	if _, err := svc.Create(ctxT(), contracts.CreateCannedResponse{
		Shortcut: "/greeting", Title: "Greeting", Body: "Hello there!", SectorIDs: []string{"sales"},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Full-scope actor resolves it and gets the body.
	allCtx := authz.WithAuthContext(ctxT(), authz.NewAuthContext("t1", "u1", nil, nil, authz.ScopeAll))
	cr, err := svc.Resolve(allCtx, "greeting")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cr.Body != "Hello there!" {
		t.Errorf("unexpected body: %q", cr.Body)
	}

	// An agent in a different sector cannot resolve it.
	otherSector := authz.WithAuthContext(ctxT(), authz.NewAuthContext("t1", "u2", nil, []string{"support"}, authz.ScopeOwn))
	if _, err := svc.Resolve(otherSector, "greeting"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("expected not_found for out-of-sector agent, got %v", err)
	}

	// An agent in the right sector can.
	rightSector := authz.WithAuthContext(ctxT(), authz.NewAuthContext("t1", "u3", nil, []string{"sales"}, authz.ScopeOwn))
	if _, err := svc.Resolve(rightSector, "greeting"); err != nil {
		t.Errorf("expected sales agent to resolve, got %v", err)
	}
}

func TestCanned_DuplicateShortcutRejected(t *testing.T) {
	svc := NewCannedResponseService(newCannedRepo(), fixedClock{})
	if _, err := svc.Create(ctxT(), contracts.CreateCannedResponse{Shortcut: "hi", Body: "x"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Create(ctxT(), contracts.CreateCannedResponse{Shortcut: "/HI", Body: "y"}); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("expected conflict for duplicate shortcut, got %v", err)
	}
}

func TestCloseReason_RequiresNotePolicy(t *testing.T) {
	svc := NewCloseReasonService(newCloseRepo(), fixedClock{t: time.Unix(1700000000, 0).UTC()})
	yes := true
	cr, err := svc.Create(ctxT(), contracts.CreateCloseReason{Name: "Spam", RequiresNote: &yes})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.RequiresNote(ctxT(), cr.ID)
	if err != nil {
		t.Fatalf("requires note: %v", err)
	}
	if !got {
		t.Errorf("expected requires_note=true")
	}
	// Unknown reason → not_found.
	if _, err := svc.RequiresNote(ctxT(), "ghost"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("expected not_found for unknown reason, got %v", err)
	}
}
