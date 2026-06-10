// Package http hosts the infrastructure-level HTTP endpoints that are not tied
// to a business domain (health, readiness).
package http

import (
	"net/http"

	"github.com/romerito007/chat-smsnet-omnichannel/app/health"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	checker *health.Checker
}

// NewHealthHandler builds the handler.
func NewHealthHandler(checker *health.Checker) *HealthHandler {
	return &HealthHandler{checker: checker}
}

// Live is a cheap liveness probe: the process is up and serving.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready probes dependencies and returns 503 when any is unavailable.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	report := h.checker.Check(r.Context())
	status := http.StatusOK
	if report.Status != health.StatusOK {
		status = http.StatusServiceUnavailable
	}
	middleware.WriteJSON(w, status, report)
}
