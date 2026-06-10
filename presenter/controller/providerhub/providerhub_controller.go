// Package providerhub holds the HTTP controllers for the providerhub config and
// the on-demand conversation queries.
package providerhub

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	phservice "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/providerhub"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves config management and on-demand provider queries.
type Controller struct {
	config  *phservice.ConfigService
	queries *phservice.QueryService
}

// NewController builds the controller.
func NewController(config *phservice.ConfigService, queries *phservice.QueryService) *Controller {
	return &Controller{config: config, queries: queries}
}

// ── config ───────────────────────────────────────────────────────────────────

// GetConfig handles GET /v1/providerhub/config.
func (c *Controller) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := c.config.Current(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConfigResponse(cfg))
}

// CreateConfig handles POST /v1/providerhub/config.
func (c *Controller) CreateConfig(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateConfigRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	cfg, err := c.config.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewConfigResponse(cfg))
}

// UpdateConfig handles PATCH /v1/providerhub/config.
func (c *Controller) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateConfigRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	cfg, err := c.config.Update(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewConfigResponse(cfg))
}

// TestConfig handles POST /v1/providerhub/config/test.
func (c *Controller) TestConfig(w http.ResponseWriter, r *http.Request) {
	result, err := c.config.Test(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, result)
}

// ── on-demand conversation queries ───────────────────────────────────────────

func (c *Controller) CustomerProfile(w http.ResponseWriter, r *http.Request) {
	res, err := c.queries.CustomerProfile(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

func (c *Controller) Contracts(w http.ResponseWriter, r *http.Request) {
	res, err := c.queries.Contracts(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"data": res})
}

func (c *Controller) FinancialStatus(w http.ResponseWriter, r *http.Request) {
	res, err := c.queries.FinancialStatus(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

func (c *Controller) ConnectionStatus(w http.ResponseWriter, r *http.Request) {
	res, err := c.queries.ConnectionStatus(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, res)
}

func (c *Controller) Tickets(w http.ResponseWriter, r *http.Request) {
	res, err := c.queries.Tickets(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"data": res})
}

func (c *Controller) OpenTicket(w http.ResponseWriter, r *http.Request) {
	var req dto.OpenTicketRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	res, err := c.queries.OpenTicket(r.Context(), chi.URLParam(r, "id"), req.ToInput())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, res)
}
