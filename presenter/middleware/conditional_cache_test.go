package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// jsonHandler writes a fixed JSON body via WriteJSON (the real response path).
func jsonHandler(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"body": body})
	})
}

func TestConditionalCache_FirstHitThen304(t *testing.T) {
	h := ConditionalCache(45 * time.Second)(jsonHandler("tenant-a-catalog"))

	// First call: 200 + ETag + Cache-Control + body.
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/v1/tags", nil))
	if rec1.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", rec1.Code)
	}
	etag := rec1.Header().Get("ETag")
	if etag == "" || etag[0] != '"' {
		t.Fatalf("expected a strong (quoted) ETag, got %q", etag)
	}
	if cc := rec1.Header().Get("Cache-Control"); cc != "private, max-age=45" {
		t.Errorf("Cache-Control = %q, want private, max-age=45", cc)
	}
	if rec1.Body.Len() == 0 {
		t.Errorf("first response must carry the body")
	}

	// Second call with the ETag: 304, no body.
	req2 := httptest.NewRequest(http.MethodGet, "/v1/tags", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("conditional status = %d, want 304", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Errorf("304 must have no body, got %q", rec2.Body.String())
	}
	if rec2.Header().Get("ETag") != etag {
		t.Errorf("304 must echo the ETag")
	}

	// A stale ETag still gets the full 200 body.
	req3 := httptest.NewRequest(http.MethodGet, "/v1/tags", nil)
	req3.Header.Set("If-None-Match", `"stale"`)
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK || rec3.Body.Len() == 0 {
		t.Errorf("stale ETag must return 200 + body, got %d len=%d", rec3.Code, rec3.Body.Len())
	}
}

func TestConditionalCache_ETagChangesWithBodyAndTenant(t *testing.T) {
	etagFor := func(body string) string {
		h := ConditionalCache(30 * time.Second)(jsonHandler(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tags", nil))
		return rec.Header().Get("ETag")
	}
	// Resource change (different content) → different ETag (invalidation).
	if etagFor("v1") == etagFor("v2") {
		t.Errorf("a changed resource must produce a different ETag")
	}
	// Tenant isolation: different tenant catalogs (different bodies) → different
	// ETags, so tenant A's If-None-Match never 304s against tenant B's body.
	if etagFor("tenant-a") == etagFor("tenant-b") {
		t.Errorf("different tenants must produce different ETags")
	}
	// Identical content → identical ETag (deterministic).
	a, b := etagFor("same"), etagFor("same")
	if a != b {
		t.Errorf("identical content must produce a stable ETag")
	}
}

// Non-GET and non-200 responses are passed through without validators.
func TestConditionalCache_OnlyCachesSuccessfulGets(t *testing.T) {
	// POST is not cached.
	postH := ConditionalCache(45 * time.Second)(jsonHandler("x"))
	rec := httptest.NewRecorder()
	postH.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/tags", nil))
	if rec.Header().Get("ETag") != "" || rec.Header().Get("Cache-Control") != "" {
		t.Errorf("non-GET must not carry cache validators")
	}

	// A non-200 GET is not cached.
	errH := ConditionalCache(45 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": "no"})
	}))
	rec2 := httptest.NewRecorder()
	errH.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/v1/tags", nil))
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec2.Code)
	}
	if rec2.Header().Get("ETag") != "" {
		t.Errorf("a 403 must not be cached")
	}
}
