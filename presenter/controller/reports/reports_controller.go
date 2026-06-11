// Package reports holds the HTTP controllers for the operational reports. Every
// endpoint requires report.view; filters come from the query string (period +
// sector/agent/channel).
package reports

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	rcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the report endpoints.
type Controller struct {
	svc   rcontracts.ReportService
	files rcontracts.FileStore
}

// NewController builds the controller.
func NewController(svc rcontracts.ReportService, files rcontracts.FileStore) *Controller {
	return &Controller{svc: svc, files: files}
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
// It renders the report into a real file and returns a temporary signed URL.
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
	res, err := c.svc.Export(r.Context(), report, format, filter(r))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

// Download handles GET /v1/reports/downloads/{token}. Public: the unguessable,
// expiring, HMAC-signed token is the only credential, mirroring the privacy and
// CSAT public-token model.
func (c *Controller) Download(w http.ResponseWriter, r *http.Request) {
	key, err := c.files.Resolve(chi.URLParam(r, "token"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	data, filename, err := c.files.Open(key)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", contentTypeFor(filename))
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func contentTypeFor(filename string) string {
	if strings.HasSuffix(filename, ".csv") {
		return "text/csv; charset=utf-8"
	}
	return "application/json; charset=utf-8"
}

func write(w http.ResponseWriter, r *http.Request, res any, err error) {
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}
