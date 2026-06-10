package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/app/health"
)

// With nil dependencies the checker skips every probe and reports OK, so both
// probes can be exercised without Mongo/Redis.
func newTestHandler() *HealthHandler {
	return NewHealthHandler(health.NewChecker(nil, nil))
}

func TestLiveReturnsOK(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler().Live(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status field = %q, want ok", body["status"])
	}
}

func TestReadyReturnsOKWhenNoDeps(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler().Ready(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var report health.Report
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if report.Status != health.StatusOK {
		t.Fatalf("report status = %q, want ok", report.Status)
	}
}
