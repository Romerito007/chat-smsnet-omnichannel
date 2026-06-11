package openapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestHandler_DevPublicServesJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	Handler(Config{Public: true}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type = %q", ct)
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Errorf("served openapi version = %v", doc["openapi"])
	}
}

func TestHandler_ProdRequiresBasicAuth(t *testing.T) {
	h := Handler(Config{Public: false, BasicUser: "docs", BasicPass: "s3cret"})

	// No credentials → 401.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-auth status = %d, want 401", rec.Code)
	}

	// Valid credentials → 200.
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	req.SetBasicAuth("docs", "s3cret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authed status = %d, want 200", rec.Code)
	}
}

func TestHandler_ProdLockedWhenUnconfigured(t *testing.T) {
	rec := httptest.NewRecorder()
	Handler(Config{Public: false}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (locked when no creds in prod)", rec.Code)
	}
}

// TestDocsCopyInSync fails if docs/openapi.yaml is stale relative to the spec, so
// the committed contract never drifts. Regenerate with `go run ./cmd/openapigen`.
func TestDocsCopyInSync(t *testing.T) {
	const docsPath = "../../docs/openapi.yaml"
	committed, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("read %s: %v (run: go run ./cmd/openapigen)", docsPath, err)
	}
	current, err := YAML()
	if err != nil {
		t.Fatalf("marshal yaml: %v", err)
	}
	if string(committed) != string(current) {
		t.Fatalf("%s is stale; regenerate with: go run ./cmd/openapigen", docsPath)
	}
	// And it must be parseable YAML describing OpenAPI 3.1.
	var doc map[string]any
	if err := yaml.Unmarshal(committed, &doc); err != nil {
		t.Fatalf("docs/openapi.yaml is not valid YAML: %v", err)
	}
	if doc["openapi"] != "3.1.0" {
		t.Errorf("docs openapi version = %v", doc["openapi"])
	}
}
