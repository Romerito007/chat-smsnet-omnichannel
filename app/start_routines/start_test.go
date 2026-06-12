package start_routines

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// tagHandler writes its tag so the test can tell which handler served a request.
func tagHandler(tag string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(tag)) }
}

// TestComposeRouter_WSReachableAtBothPaths reproduces the role=all wiring and
// guards the fix for the WS 404: the realtime handler must be reachable at both
// /ws and /realtime/ws, while every other path falls through to the API.
func TestComposeRouter_WSReachableAtBothPaths(t *testing.T) {
	h := composeRouter(tagHandler("api"), tagHandler("ws"))

	cases := []struct{ path, want string }{
		{"/ws", "ws"},
		{"/realtime/ws", "ws"},
		{"/v1/me", "api"},
		{"/healthz", "api"},
		{"/openapi.json", "api"},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", tc.path, rec.Code)
		}
		if rec.Body.String() != tc.want {
			t.Errorf("%s: served %q, want %q", tc.path, rec.Body.String(), tc.want)
		}
	}
}

// API-only mode: no WS handler, so /ws falls through to the API (no panic).
func TestComposeRouter_APIOnly(t *testing.T) {
	h := composeRouter(tagHandler("api"), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/me", nil))
	if rec.Body.String() != "api" {
		t.Errorf("API-only: served %q, want api", rec.Body.String())
	}
}

// WS-only mode: the WS paths work without an API mount.
func TestComposeRouter_WSOnly(t *testing.T) {
	h := composeRouter(nil, tagHandler("ws"))
	for _, p := range []string{"/ws", "/realtime/ws"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Body.String() != "ws" {
			t.Errorf("WS-only %s: served %q, want ws", p, rec.Body.String())
		}
	}
}
