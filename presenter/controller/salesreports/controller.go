// Package salesreports holds the HTTP controller for the sales-funnel metrics
// (GET /v1/reports/sales/*). Backed by the deals SalesMetrics service; every
// endpoint requires report.view and respects the actor's deal visibility.
package salesreports

import (
	"net/http"
	"strconv"
	"time"

	dcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/deals/contracts"
	dealservice "github.com/romerito007/chat-smsnet-omnichannel/domain/deals/service"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the sales metrics.
type Controller struct {
	metrics *dealservice.SalesMetrics
}

// NewController builds the controller.
func NewController(metrics *dealservice.SalesMetrics) *Controller {
	return &Controller{metrics: metrics}
}

// Funnel handles GET /v1/reports/sales/funnel.
func (c *Controller) Funnel(w http.ResponseWriter, r *http.Request) {
	res, err := c.metrics.Funnel(r.Context(), filter(r))
	write(w, r, res, err)
}

// Agents handles GET /v1/reports/sales/agents (seller ranking).
func (c *Controller) Agents(w http.ResponseWriter, r *http.Request) {
	res, err := c.metrics.Agents(r.Context(), filter(r))
	write(w, r, res, err)
}

// Cycle handles GET /v1/reports/sales/cycle (cycle time + stalled deals). The
// stalled threshold defaults to 14 days; override with ?stalled_days=.
func (c *Controller) Cycle(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("stalled_days"))
	res, err := c.metrics.Cycle(r.Context(), filter(r), days)
	write(w, r, res, err)
}

func filter(r *http.Request) dcontracts.SalesFilter {
	q := r.URL.Query()
	return dcontracts.SalesFilter{
		PipelineID: q.Get("pipeline_id"),
		From:       parseTime(q.Get("from")),
		To:         parseTime(q.Get("to")),
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

func write(w http.ResponseWriter, r *http.Request, res any, err error) {
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}
