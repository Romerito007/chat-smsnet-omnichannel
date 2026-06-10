package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth/contracts"
	authentity "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
)

// ── test doubles (interface embedding keeps them minimal) ────────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeHasher struct{}

func (fakeHasher) Hash(plain string) (string, error) { return "hashed:" + plain, nil }
func (fakeHasher) Compare(hash, plain string) error {
	if hash == "hashed:"+plain {
		return nil
	}
	return apperror.Unauthorized("mismatch")
}

type fakeUserRepo struct {
	iamrepo.UserRepository
	byEmail map[string]*iamentity.User
	byID    map[string]*iamentity.User
}

func (r *fakeUserRepo) FindByEmailAnyTenant(_ context.Context, email string) (*iamentity.User, error) {
	if u, ok := r.byEmail[email]; ok {
		return u, nil
	}
	return nil, apperror.NotFound("resource not found")
}
func (r *fakeUserRepo) FindByID(_ context.Context, id string) (*iamentity.User, error) {
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, apperror.NotFound("resource not found")
}

type fakeRoleRepo struct {
	iamrepo.RoleRepository
	roles map[string]*iamentity.Role
}

func (r *fakeRoleRepo) FindByIDs(_ context.Context, ids []string) ([]*iamentity.Role, error) {
	var out []*iamentity.Role
	for _, id := range ids {
		if role, ok := r.roles[id]; ok {
			out = append(out, role)
		}
	}
	return out, nil
}

type fakeRefreshRepo struct {
	tokens map[string]*authentity.RefreshToken // keyed by hash
}

func (r *fakeRefreshRepo) Create(_ context.Context, t *authentity.RefreshToken) error {
	r.tokens[t.TokenHash] = t
	return nil
}
func (r *fakeRefreshRepo) FindByHash(_ context.Context, hash string) (*authentity.RefreshToken, error) {
	if t, ok := r.tokens[hash]; ok {
		return t, nil
	}
	return nil, apperror.NotFound("resource not found")
}
func (r *fakeRefreshRepo) Revoke(_ context.Context, id string) error {
	for _, t := range r.tokens {
		if t.ID == id {
			now := time.Unix(1700000000, 0).UTC()
			t.RevokedAt = &now
		}
	}
	return nil
}
func (r *fakeRefreshRepo) RevokeAllForUser(_ context.Context, _, _ string) error { return nil }

type fakeTokenManager struct{ counter int }

func (m *fakeTokenManager) IssueAccess(claims auth.AccessClaims) (string, time.Time, error) {
	return "access-" + claims.UserID, time.Unix(1700000900, 0).UTC(), nil
}
func (m *fakeTokenManager) VerifyAccess(string) (auth.AccessClaims, error) {
	return auth.AccessClaims{}, nil
}
func (m *fakeTokenManager) GenerateRefresh() (string, time.Time, error) {
	m.counter++
	return fmt.Sprintf("refresh-%d", m.counter), time.Unix(1700086400, 0).UTC(), nil
}
func (m *fakeTokenManager) HashRefresh(plaintext string) string { return "h:" + plaintext }

// ── fixture ──────────────────────────────────────────────────────────────────

func newAuthService() (*Service, *fakeRefreshRepo) {
	owner := &iamentity.User{
		ID:           "u1",
		TenantID:     "t1",
		Email:        "owner@example.com",
		PasswordHash: "hashed:correct-horse",
		Status:       iamentity.StatusActive,
		RoleIDs:      []string{"r1"},
		SectorIDs:    []string{"s1"},
	}
	users := &fakeUserRepo{
		byEmail: map[string]*iamentity.User{owner.Email: owner},
		byID:    map[string]*iamentity.User{owner.ID: owner},
	}
	roles := &fakeRoleRepo{roles: map[string]*iamentity.Role{
		"r1": {ID: "r1", TenantID: "t1", Name: "owner",
			Permissions: []authz.Permission{authz.ConversationRead, authz.UserManage},
			SectorScope: authz.ScopeAll},
	}}
	refresh := &fakeRefreshRepo{tokens: map[string]*authentity.RefreshToken{}}
	clock := fixedClock{t: time.Unix(1700000000, 0).UTC()}
	svc := New(users, roles, refresh, fakeHasher{}, &fakeTokenManager{}, clock)
	return svc, refresh
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	svc, refresh := newAuthService()
	pair, err := svc.Login(context.Background(), contracts.LoginCommand{
		Email: "owner@example.com", Password: "correct-horse",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if pair.AccessToken != "access-u1" {
		t.Errorf("access token = %q", pair.AccessToken)
	}
	if pair.RefreshToken == "" {
		t.Error("missing refresh token")
	}
	if pair.SectorScope != authz.ScopeAll {
		t.Errorf("scope = %q, want all", pair.SectorScope)
	}
	if len(pair.Permissions) != 2 {
		t.Errorf("permissions = %d, want 2", len(pair.Permissions))
	}
	if len(refresh.tokens) != 1 {
		t.Errorf("refresh token not persisted (%d)", len(refresh.tokens))
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, _ := newAuthService()
	_, err := svc.Login(context.Background(), contracts.LoginCommand{
		Email: "owner@example.com", Password: "wrong",
	})
	if !isCode(err, apperror.CodeUnauthorized) {
		t.Errorf("expected unauthorized, got %v", err)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc, _ := newAuthService()
	_, err := svc.Login(context.Background(), contracts.LoginCommand{
		Email: "nobody@example.com", Password: "correct-horse",
	})
	if !isCode(err, apperror.CodeUnauthorized) {
		t.Errorf("expected unauthorized, got %v", err)
	}
}

func TestRefresh_RotatesAndInvalidatesOld(t *testing.T) {
	svc, _ := newAuthService()
	pair, err := svc.Login(context.Background(), contracts.LoginCommand{
		Email: "owner@example.com", Password: "correct-horse",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	next, err := svc.Refresh(context.Background(), contracts.RefreshCommand{RefreshToken: pair.RefreshToken})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if next.RefreshToken == pair.RefreshToken {
		t.Error("refresh token was not rotated")
	}

	// The old token must no longer be accepted.
	if _, err := svc.Refresh(context.Background(), contracts.RefreshCommand{RefreshToken: pair.RefreshToken}); !isCode(err, apperror.CodeUnauthorized) {
		t.Errorf("expected unauthorized for reused token, got %v", err)
	}
}

func TestLogout_RevokesAndIsIdempotent(t *testing.T) {
	svc, _ := newAuthService()
	pair, err := svc.Login(context.Background(), contracts.LoginCommand{
		Email: "owner@example.com", Password: "correct-horse",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if err := svc.Logout(context.Background(), contracts.LogoutCommand{RefreshToken: pair.RefreshToken}); err != nil {
		t.Fatalf("logout: %v", err)
	}
	// Idempotent: logging out an unknown token is not an error.
	if err := svc.Logout(context.Background(), contracts.LogoutCommand{RefreshToken: "nope"}); err != nil {
		t.Errorf("logout idempotency: %v", err)
	}
	// The revoked token cannot be refreshed.
	if _, err := svc.Refresh(context.Background(), contracts.RefreshCommand{RefreshToken: pair.RefreshToken}); !isCode(err, apperror.CodeUnauthorized) {
		t.Errorf("expected unauthorized after logout, got %v", err)
	}
}

func isCode(err error, code apperror.Code) bool {
	appErr := apperror.From(err)
	return appErr != nil && appErr.Code == code
}
