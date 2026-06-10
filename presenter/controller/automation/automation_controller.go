// Package automation holds the HTTP controllers for the automation endpoints.
package automation

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	autorepo "github.com/romerito007/chat-smsnet-omnichannel/domain/automation/repository"
	automationservice "github.com/romerito007/chat-smsnet-omnichannel/domain/automation/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/automation"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

const maxCallbackBody = 1 << 20

// Controller serves integration CRUD, run reads and the public callback.
type Controller struct {
	integrations *automationservice.IntegrationService
	automation   *automationservice.Service
	runs         autorepo.RunRepository
}

// NewController builds the controller.
func NewController(integrations *automationservice.IntegrationService, automation *automationservice.Service, runs autorepo.RunRepository) *Controller {
	return &Controller{integrations: integrations, automation: automation, runs: runs}
}

// ── integration CRUD (channel-manage gated by the route) ─────────────────────

func (c *Controller) CreateIntegration(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateIntegrationRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	it, err := c.integrations.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewIntegrationResponse(it))
}

func (c *Controller) ListIntegrations(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.integrations.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewIntegrationResponses(items), page.Limit, func(it dto.IntegrationResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: it.CreatedAt.UnixMilli(), ID: it.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (c *Controller) GetIntegration(w http.ResponseWriter, r *http.Request) {
	it, err := c.integrations.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewIntegrationResponse(it))
}

func (c *Controller) UpdateIntegration(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateIntegrationRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	it, err := c.integrations.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewIntegrationResponse(it))
}

func (c *Controller) DeleteIntegration(w http.ResponseWriter, r *http.Request) {
	if err := c.integrations.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── runs ─────────────────────────────────────────────────────────────────────

func (c *Controller) ListRuns(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.runs.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewRunResponses(items), page.Limit, func(it dto.RunResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: it.CreatedAt.UnixMilli(), ID: it.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (c *Controller) GetRun(w http.ResponseWriter, r *http.Request) {
	run, err := c.runs.FindByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewRunResponse(run))
}

// ── callback (public, signature-verified) ────────────────────────────────────

// Callback handles POST /v1/automation/callbacks/{tenant_id}.
func (c *Controller) Callback(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenant_id")
	body, err := io.ReadAll(io.LimitReader(r.Body, maxCallbackBody))
	if err != nil {
		middleware.WriteError(w, r, apperror.Validation("unreadable request body"))
		return
	}
	if err := c.automation.HandleCallback(r.Context(), tenantID, body, r.Header.Get("X-Signature")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
