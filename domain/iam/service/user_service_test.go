package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── test doubles ─────────────────────────────────────────────────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// fakeHasher is a deterministic, fast stand-in for bcrypt.
type fakeHasher struct{}

func (fakeHasher) Hash(plain string) (string, error) { return "hashed:" + plain, nil }
func (fakeHasher) Compare(hash, plain string) error {
	if hash == "hashed:"+plain {
		return nil
	}
	return apperror.Unauthorized("mismatch")
}

// fakeUserRepo is an in-memory user repository that honors tenant scope from the
// context, mirroring the real Mongo implementation's isolation.
type fakeUserRepo struct {
	users map[string]*entity.User
}

func newFakeUserRepo() *fakeUserRepo { return &fakeUserRepo{users: map[string]*entity.User{}} }

func (r *fakeUserRepo) Create(ctx context.Context, u *entity.User) error {
	for _, ex := range r.users {
		if ex.TenantID == u.TenantID && ex.Email == u.Email {
			return apperror.Conflict("resource already exists")
		}
	}
	cp := *u
	r.users[u.ID] = &cp
	return nil
}

func (r *fakeUserRepo) Update(ctx context.Context, u *entity.User) error {
	if _, ok := r.users[u.ID]; !ok {
		return apperror.NotFound("resource not found")
	}
	cp := *u
	r.users[u.ID] = &cp
	return nil
}

func (r *fakeUserRepo) Delete(ctx context.Context, id string) error {
	if _, ok := r.users[id]; !ok {
		return apperror.NotFound("resource not found")
	}
	delete(r.users, id)
	return nil
}

func (r *fakeUserRepo) FindByID(ctx context.Context, id string) (*entity.User, error) {
	tenant, _ := shared.TenantFrom(ctx)
	if u, ok := r.users[id]; ok && u.TenantID == tenant {
		cp := *u
		return &cp, nil
	}
	return nil, apperror.NotFound("resource not found")
}

func (r *fakeUserRepo) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	tenant, _ := shared.TenantFrom(ctx)
	for _, u := range r.users {
		if u.TenantID == tenant && u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, apperror.NotFound("resource not found")
}

func (r *fakeUserRepo) FindByEmailAnyTenant(ctx context.Context, email string) (*entity.User, error) {
	for _, u := range r.users {
		if u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, apperror.NotFound("resource not found")
}

func (r *fakeUserRepo) List(ctx context.Context, page shared.PageRequest) ([]*entity.User, error) {
	tenant, _ := shared.TenantFrom(ctx)
	var out []*entity.User
	for _, u := range r.users {
		if u.TenantID == tenant {
			cp := *u
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeUserRepo) ListBySector(ctx context.Context, sectorID string) ([]*entity.User, error) {
	tenant, _ := shared.TenantFrom(ctx)
	var out []*entity.User
	for _, u := range r.users {
		if u.TenantID != tenant {
			continue
		}
		for _, s := range u.SectorIDs {
			if s == sectorID {
				cp := *u
				out = append(out, &cp)
			}
		}
	}
	return out, nil
}

func tenantCtx(tenant string) context.Context {
	return shared.WithTenant(context.Background(), tenant)
}

// ── tests ────────────────────────────────────────────────────────────────────

func newUserService() (*UserService, *fakeUserRepo) {
	repo := newFakeUserRepo()
	svc := NewUserService(repo, fakeHasher{}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	return svc, repo
}

func TestCreateUser_HashesAndScopes(t *testing.T) {
	svc, repo := newUserService()
	ctx := tenantCtx("t1")

	u, err := svc.Create(ctx, contracts.CreateUser{
		Name:     "Alice",
		Email:    "Alice@Example.com",
		Password: "supersecret",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.TenantID != "t1" {
		t.Errorf("tenant = %q, want t1", u.TenantID)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("email not normalized: %q", u.Email)
	}
	if u.PasswordHash != "hashed:supersecret" {
		t.Errorf("password not hashed: %q", u.PasswordHash)
	}
	if repo.users[u.ID] == nil {
		t.Error("user not persisted")
	}
}

func TestCreateUser_NormalizesSectorIDs(t *testing.T) {
	svc, repo := newUserService()
	ctx := tenantCtx("t1")

	// Dirty input: an empty string and a duplicate (the [""] / junk shape the
	// broken seed produced) must never reach storage.
	u, err := svc.Create(ctx, contracts.CreateUser{
		Name: "Bruno", Email: "bruno@demo.local", Password: "supersecret",
		SectorIDs: []string{"", "s1", "s1", "  "},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(u.SectorIDs) != 1 || u.SectorIDs[0] != "s1" {
		t.Fatalf("sector_ids not normalized: %v", u.SectorIDs)
	}
	if got := repo.users[u.ID].SectorIDs; len(got) != 1 || got[0] != "s1" {
		t.Errorf("persisted sector_ids not normalized: %v", got)
	}

	// "Sem setor" is the empty slice, never nil.
	u2, err := svc.Create(ctx, contracts.CreateUser{
		Name: "Ana", Email: "ana@demo.local", Password: "supersecret", SectorIDs: nil,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u2.SectorIDs == nil {
		t.Errorf("expected empty (non-nil) sector_ids, got nil")
	}
}

func TestCreateUser_Validation(t *testing.T) {
	svc, _ := newUserService()
	ctx := tenantCtx("t1")

	cases := map[string]contracts.CreateUser{
		"bad email":      {Name: "A", Email: "nope", Password: "supersecret"},
		"short password": {Name: "A", Email: "a@b.com", Password: "short"},
		"missing name":   {Name: "", Email: "a@b.com", Password: "supersecret"},
	}
	for name, cmd := range cases {
		if _, err := svc.Create(ctx, cmd); !isValidation(err) {
			t.Errorf("%s: expected validation_error, got %v", name, err)
		}
	}
}

func TestCreateUser_DuplicateEmailConflicts(t *testing.T) {
	svc, _ := newUserService()
	ctx := tenantCtx("t1")
	cmd := contracts.CreateUser{Name: "A", Email: "a@b.com", Password: "supersecret"}
	if _, err := svc.Create(ctx, cmd); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(ctx, cmd)
	if !isCode(err, apperror.CodeConflict) {
		t.Errorf("expected conflict, got %v", err)
	}
}

func TestCreateUser_RequiresTenant(t *testing.T) {
	svc, _ := newUserService()
	_, err := svc.Create(context.Background(), contracts.CreateUser{Name: "A", Email: "a@b.com", Password: "supersecret"})
	if !isCode(err, apperror.CodeForbidden) {
		t.Errorf("expected forbidden without tenant, got %v", err)
	}
}

func TestGetUser_TenantIsolation(t *testing.T) {
	svc, _ := newUserService()
	u, err := svc.Create(tenantCtx("t1"), contracts.CreateUser{Name: "A", Email: "a@b.com", Password: "supersecret"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Another tenant must not see it.
	if _, err := svc.Get(tenantCtx("t2"), u.ID); !isCode(err, apperror.CodeNotFound) {
		t.Errorf("expected not_found cross-tenant, got %v", err)
	}
	if _, err := svc.Get(tenantCtx("t1"), u.ID); err != nil {
		t.Errorf("same-tenant get failed: %v", err)
	}
}

func isValidation(err error) bool { return isCode(err, apperror.CodeValidation) }

func isCode(err error, code apperror.Code) bool {
	appErr := apperror.From(err)
	return appErr != nil && appErr.Code == code
}

func TestListBySector_OnlySectorMembers(t *testing.T) {
	svc, repo := newUserService()
	ctx := tenantCtx("t1")
	repo.users["a"] = &entity.User{ID: "a", TenantID: "t1", Status: entity.StatusActive, SectorIDs: []string{"s1", "s2"}}
	repo.users["b"] = &entity.User{ID: "b", TenantID: "t1", Status: entity.StatusActive, SectorIDs: []string{"s2"}}
	repo.users["c"] = &entity.User{ID: "c", TenantID: "t2", Status: entity.StatusActive, SectorIDs: []string{"s1"}} // other tenant

	got, err := svc.ListBySector(ctx, "s1")
	if err != nil {
		t.Fatalf("list by sector: %v", err)
	}
	if len(got) != 1 || got[0].ID != "a" {
		t.Errorf("want only agent a (member of s1, tenant t1), got %+v", got)
	}
}
