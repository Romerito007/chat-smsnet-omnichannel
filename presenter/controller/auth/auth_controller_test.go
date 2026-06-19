package auth_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	authentity "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/entity"
	authrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/repository"
	authservice "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/service"
	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/security"
	authctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/auth"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/httpharness"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

var (
	tm     = httpharness.Tokens()
	hasher = security.NewBcryptHasher(4) // low cost for fast tests
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fakeUsers struct{ byEmail map[string]*iamentity.User }

func (r *fakeUsers) Create(context.Context, *iamentity.User) error { return nil }
func (r *fakeUsers) Update(context.Context, *iamentity.User) error { return nil }
func (r *fakeUsers) Delete(context.Context, string) error          { return nil }
func (r *fakeUsers) FindByID(_ context.Context, id string) (*iamentity.User, error) {
	for _, u := range r.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, apperror.NotFound("none")
}
func (r *fakeUsers) FindByIDs(_ context.Context, ids []string) ([]*iamentity.User, error) {
	var out []*iamentity.User
	for _, id := range ids {
		for _, u := range r.byEmail {
			if u.ID == id {
				out = append(out, u)
			}
		}
	}
	return out, nil
}
func (r *fakeUsers) FindByEmail(context.Context, string) (*iamentity.User, error) {
	return nil, apperror.NotFound("none")
}
func (r *fakeUsers) FindByEmailAnyTenant(_ context.Context, email string) (*iamentity.User, error) {
	if u, ok := r.byEmail[email]; ok {
		return u, nil
	}
	return nil, apperror.NotFound("none")
}
func (r *fakeUsers) List(context.Context, shared.PageRequest) ([]*iamentity.User, error) {
	return nil, nil
}
func (r *fakeUsers) ListBySector(context.Context, string) ([]*iamentity.User, error) { return nil, nil }
func (r *fakeUsers) SetPresenceSettings(context.Context, string, *string, *bool) error {
	return nil
}

type fakeRoles struct{}

func (fakeRoles) Create(context.Context, *iamentity.Role) error { return nil }
func (fakeRoles) Update(context.Context, *iamentity.Role) error { return nil }
func (fakeRoles) Delete(context.Context, string) error          { return nil }
func (fakeRoles) FindByID(context.Context, string) (*iamentity.Role, error) {
	return nil, apperror.NotFound("none")
}
func (fakeRoles) FindByIDs(context.Context, []string) ([]*iamentity.Role, error) { return nil, nil }
func (fakeRoles) FindByName(context.Context, string) (*iamentity.Role, error) {
	return nil, apperror.NotFound("none")
}
func (fakeRoles) List(context.Context, shared.PageRequest) ([]*iamentity.Role, error) {
	return nil, nil
}

type fakeTokens struct{}

func (fakeTokens) Create(context.Context, *authentity.RefreshToken) error { return nil }
func (fakeTokens) FindByHash(context.Context, string) (*authentity.RefreshToken, error) {
	return nil, apperror.NotFound("none")
}
func (fakeTokens) Revoke(context.Context, string) error                   { return nil }
func (fakeTokens) RevokeAllForUser(context.Context, string, string) error { return nil }

var (
	_ iamrepo.UserRepository          = (*fakeUsers)(nil)
	_ iamrepo.RoleRepository          = fakeRoles{}
	_ authrepo.RefreshTokenRepository = fakeTokens{}
)

// ── harness ──────────────────────────────────────────────────────────────────

func buildRouter(users *fakeUsers) http.Handler {
	authSvc := authservice.New(users, fakeRoles{}, fakeTokens{}, hasher, tm, shared.SystemClock{})
	userSvc := iamservice.NewUserService(users, hasher, shared.SystemClock{})
	ctl := authctl.NewController(authSvc, userSvc)

	r := chi.NewRouter()
	r.Post("/auth/login", ctl.Login)
	r.Group(func(p chi.Router) {
		p.Use(middleware.AuthContext(tm))
		p.Get("/me", ctl.Me)
	})
	return r
}

func seedUser(t *testing.T) *fakeUsers {
	t.Helper()
	hash, err := hasher.Hash("secret123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return &fakeUsers{byEmail: map[string]*iamentity.User{
		"a@x.com": {ID: "u1", TenantID: "t1", Email: "a@x.com", PasswordHash: hash, Status: iamentity.StatusActive},
	}}
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestAuth_Login_HappyReturnsTokenPair(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(seedUser(t)), http.MethodPost, "/auth/login", "",
		map[string]any{"email": "a@x.com", "password": "secret123"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	httpharness.DecodeJSON(t, rec, &resp)
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Errorf("expected an access + refresh token, got %+v", resp)
	}
}

func TestAuth_Login_WrongPassword_401(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(seedUser(t)), http.MethodPost, "/auth/login", "",
		map[string]any{"email": "a@x.com", "password": "WRONG"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeUnauthorized {
		t.Errorf("code = %q, want unauthorized", code)
	}
}

func TestAuth_Login_InvalidPayload_400(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(seedUser(t)), http.MethodPost, "/auth/login", "", "{bad")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
	if code := httpharness.ErrorCode(t, rec); code != apperror.CodeValidation {
		t.Errorf("code = %q, want validation_error", code)
	}
}

// /me requires authentication: no token → 401 (covers the protected path).
func TestAuth_Me_NoToken_401(t *testing.T) {
	rec := httpharness.Do(t, buildRouter(seedUser(t)), http.MethodGet, "/me", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
