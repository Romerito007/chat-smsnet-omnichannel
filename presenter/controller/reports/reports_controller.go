// Package reports holds the HTTP controllers for the operational reports. Every
// endpoint requires report.view; filters come from the query string (period +
// sector/agent/channel).
package reports

import (
	"net/http"
	"time"

	rcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the report endpoints.
type Controller struct {
	svc rcontracts.ReportService
}

// NewController builds the controller.
func NewController(svc rcontracts.ReportService) *Controller {
	return &Controller{svc: svc}
}

// filter parses the common report filter from the query string.
func filter(r *http.Request) rcontracts.Filter {
	q := r.URL.Query()
	return rcontracts.Filter{
		From:       parseTime(q.Get("from")),
		To:         parseTime(q.Get("to")),
		SectorID:   q.Get("sector_id"),
		AssignedTo: q.Get("assigned_to"),
		Channel:    q.Get("channel"),
	}
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// Overview handles GET /v1/reports/overview.
func (c *Controller) Overview(w http.ResponseWriter, r *http.Request) {
	res, err := c.svc.Overview(r.Context(), filter(r))
	write(w, r, res, err)
}

// Conversations handles GET /v1/reports/conversations.
func (c *Controller) Conversations(w http.ResponseWriter, r *http.Request) {
	res, err := c.svc.Conversations(r.Context(), filter(r))
	write(w, r, res, err)
}

// Agents handles GET /v1/reports/agents.
func (c *Controller) Agents(w http.ResponseWriter, r *http.Request) {
	res, err := c.svc.Agents(r.Context(), filter(r))
	write(w, r, res, err)
}

// Sectors handles GET /v1/reports/sectors.
func (c *Controller) Sectors(w http.ResponseWriter, r *http.Request) {
	res, err := c.svc.Sectors(r.Context(), filter(r))
	write(w, r, res, err)
}

// Automation handles GET /v1/reports/automation.
func (c *Controller) Automation(w http.ResponseWriter, r *http.Request) {
	res, err := c.svc.Automation(r.Context(), filter(r))
	write(w, r, res, err)
}

// Copilot handles GET /v1/reports/copilot.
func (c *Controller) Copilot(w http.ResponseWriter, r *http.Request) {
	res, err := c.svc.Copilot(r.Context(), filter(r))
	write(w, r, res, err)
}

// SLA handles GET /v1/reports/sla.
func (c *Controller) SLA(w http.ResponseWriter, r *http.Request) {
	res, err := c.svc.SLA(r.Context(), filter(r))
	write(w, r, res, err)
}

// CSAT handles GET /v1/reports/csat.
func (c *Controller) CSAT(w http.ResponseWriter, r *http.Request) {
	res, err := c.svc.CSAT(r.Context(), filter(r))
	write(w, r, res, err)
}

// Export handles POST /v1/reports/export?report=overview&format=csv (report.export).
// It audits and enqueues the export job; file generation is asynchronous.
func (c *Controller) Export(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	report := q.Get("report")
	if report == "" {
		report = "overview"
	}
	format := q.Get("format")
	if format == "" {
		format = "csv"
	}
	if err := c.svc.RequestExport(r.Context(), report, format, filter(r)); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusAccepted, map[string]any{
		"status": "queued", "report": report, "format": format,
	})
}

func write(w http.ResponseWriter, r *http.Request, res any, err error) {
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}
