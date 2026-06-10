// Package monitoring holds the HTTP controllers for the monitoring config and
// the on-demand conversation queries.
package monitoring

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	mservice "github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/monitoring"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves config management and on-demand monitoring queries.
type Controller struct {
	config  *mservice.ConfigService
	queries *mservice.QueryService
}

// NewController builds the controller.
func NewController(config *mservice.ConfigService, queries *mservice.QueryService) *Controller {
	return &Controller{config: config, queries: queries}
}

// GetConfig handles GET /v1/monitoring/config.
func (c *Controller) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := c.config.Current(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConfigResponse(cfg))
}

// SaveConfig handles PATCH /v1/monitoring/config (upsert).
func (c *Controller) SaveConfig(w http.ResponseWriter, r *http.Request) {
	var req dto.SaveConfigRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	cfg, err := c.config.Save(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConfigResponse(cfg))
}

// TestConfig handles POST /v1/monitoring/config/test.
func (c *Controller) TestConfig(w http.ResponseWriter, r *http.Request) {
	result, err := c.config.Test(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, result)
}

// Summary handles GET /v1/conversations/{id}/external/monitoring-summary.
func (c *Controller) Summary(w http.ResponseWriter, r *http.Request) {
	res, err := c.queries.Summary(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

// Incidents handles GET /v1/conversations/{id}/external/monitoring-incidents.
func (c *Controller) Incidents(w http.ResponseWriter, r *http.Request) {
	res, err := c.queries.Incidents(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"data": res})
}
