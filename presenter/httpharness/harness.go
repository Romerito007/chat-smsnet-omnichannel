// Package httpharness provides small helpers for HTTP controller/presenter tests:
// minting real access tokens, issuing requests against a handler, and reading the
// standard error envelope. It is imported only by tests (never by production
// code), so it does not affect the shipped binary.
package httpharness

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/security"
)

// Tokens builds a real JWT manager for tests (matching the production verifier).
func Tokens() auth.TokenManager {
	return security.NewJWTManager("test-secret", "test-issuer", time.Hour, time.Hour)
}

// Token mints an access token for the given identity. Pass the permissions the
// request should carry; sector scope defaults to all sectors.
func Token(t *testing.T, tm auth.TokenManager, tenant, user string, perms ...authz.Permission) string {
	t.Helper()
	tok, _, err := tm.IssueAccess(auth.AccessClaims{
		TenantID:    tenant,
		UserID:      user,
		Permissions: perms,
		SectorScope: authz.ScopeAll,
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return tok
}

// TokenScoped mints a token with an explicit sector scope + sectors.
func TokenScoped(t *testing.T, tm auth.TokenManager, tenant, user string, scope authz.SectorScope, sectorIDs []string, perms ...authz.Permission) string {
	t.Helper()
	tok, _, err := tm.IssueAccess(auth.AccessClaims{
		TenantID:    tenant,
		UserID:      user,
		Permissions: perms,
		SectorIDs:   sectorIDs,
		SectorScope: scope,
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return tok
}

// Do issues a request against h. body may be a string (sent verbatim, e.g. to
// exercise malformed JSON), a []byte, nil, or any JSON-serializable value.
// bearer "" omits the Authorization header.
func Do(t *testing.T, h http.Handler, method, path, bearer string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	switch b := body.(type) {
	case nil:
		reader = nil
	case string:
		reader = bytes.NewBufferString(b)
	case []byte:
		reader = bytes.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// envelope mirrors the standard error response body.
type envelope struct {
	Error struct {
		Code apperror.Code `json:"code"`
	} `json:"error"`
}

// ErrorCode decodes the standard error envelope and returns its code.
func ErrorCode(t *testing.T, rec *httptest.ResponseRecorder) apperror.Code {
	t.Helper()
	var e envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode error envelope from %q: %v", rec.Body.String(), err)
	}
	if e.Error.Code == "" {
		t.Fatalf("expected an error envelope, got %q", rec.Body.String())
	}
	return e.Error.Code
}

// DecodeJSON unmarshals the response body into v.
func DecodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
}
