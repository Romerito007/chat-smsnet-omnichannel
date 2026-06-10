// Package http hosts the infrastructure-level HTTP endpoints that are not tied
// to a business domain (health, readiness).
package http

import (
	"net/http"

	"github.com/romerito007/chat-smsnet-omnichannel/app/health"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// HealthController serves liveness and readiness probes.
type HealthController struct {
	checker *health.Checker
}

// NewHealthController builds the controller.
func NewHealthController(checker *health.Checker) *HealthController {
	return &HealthController{checker: checker}
}

// Live is a cheap liveness probe: the process is up and serving.
func (c *HealthController) Live(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready probes dependencies and returns 503 when any is unavailable.
func (c *HealthController) Ready(w http.ResponseWriter, r *http.Request) {
	report := c.checker.Check(r.Context())
	status := http.StatusOK
	if report.Status != health.StatusOK {
		status = http.StatusServiceUnavailable
	}
	middleware.WriteJSON(w, status, report)
}
