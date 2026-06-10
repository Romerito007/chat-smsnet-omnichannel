package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/security"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

func newManager() *security.JWTManager {
	return security.NewJWTManager("test-secret", "chat-backend", 15*time.Minute, time.Hour)
}

func issue(t *testing.T, m *security.JWTManager, perms ...authz.Permission) string {
	t.Helper()
	token, _, err := m.IssueAccess(auth.AccessClaims{
		TenantID:    "t1",
		UserID:      "u1",
		Permissions: perms,
		SectorScope: authz.ScopeAll,
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	return token
}

// probe records what the protected handler saw.
type probe struct {
	called  bool
	tenant  string
	hasPerm bool
}

func protectedHandler(p *probe) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.called = true
		p.tenant, _ = shared.TenantFrom(r.Context())
		ac, _ := authz.FromContext(r.Context())
		p.hasPerm = ac.Has(authz.UserManage)
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuthContext_RejectsMissingToken(t *testing.T) {
	m := newManager()
	p := &probe{}
	h := middleware.AuthContext(m)(protectedHandler(p))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/me", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if p.called {
		t.Error("handler should not run without a token")
	}
}

func TestAuthContext_RejectsBadToken(t *testing.T) {
	m := newManager()
	h := middleware.AuthContext(m)(protectedHandler(&probe{}))

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAuthContext_PopulatesTenantAndPermissions(t *testing.T) {
	m := newManager()
	p := &probe{}
	h := middleware.AuthContext(m)(protectedHandler(p))

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+issue(t, m, authz.UserManage))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !p.called {
		t.Fatal("handler not called")
	}
	if p.tenant != "t1" {
		t.Errorf("tenant from token = %q, want t1", p.tenant)
	}
	if !p.hasPerm {
		t.Error("expected user.manage permission in context")
	}
}

func TestRequirePermission_Enforces(t *testing.T) {
	m := newManager()

	// Handler gated on user.manage.
	gated := func(token string) int {
		p := &probe{}
		h := middleware.AuthContext(m)(middleware.RequirePermission(authz.UserManage)(protectedHandler(p)))
		req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := gated(issue(t, m, authz.UserManage)); code != http.StatusOK {
		t.Errorf("with permission: status = %d, want 200", code)
	}
	if code := gated(issue(t, m, authz.ConversationRead)); code != http.StatusForbidden {
		t.Errorf("without permission: status = %d, want 403", code)
	}
}
