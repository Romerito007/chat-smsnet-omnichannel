// Package sla holds the HTTP controllers for SLA policies, per-conversation SLA
// status and the at-risk listing.
package sla

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	slaservice "github.com/romerito007/chat-smsnet-omnichannel/domain/sla/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/sla"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves SLA policy CRUD, conversation SLA status and at-risk.
type Controller struct {
	policies *slaservice.PolicyService
	tracking *slaservice.Service
}

// NewController builds the controller.
func NewController(policies *slaservice.PolicyService, tracking *slaservice.Service) *Controller {
	return &Controller{policies: policies, tracking: tracking}
}

// ── policies ─────────────────────────────────────────────────────────────────

func (c *Controller) ListPolicies(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.policies.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewPolicyResponses(items), page.Limit, func(p dto.PolicyResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: p.CreatedAt.UnixMilli(), ID: p.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (c *Controller) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	var req dto.CreatePolicyRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	p, err := c.policies.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewPolicyResponse(p))
}

func (c *Controller) GetPolicy(w http.ResponseWriter, r *http.Request) {
	p, err := c.policies.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewPolicyResponse(p))
}

func (c *Controller) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdatePolicyRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	p, err := c.policies.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewPolicyResponse(p))
}

func (c *Controller) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	if err := c.policies.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── tracking ─────────────────────────────────────────────────────────────────

// ConversationSLA handles GET /v1/conversations/{id}/sla.
func (c *Controller) ConversationSLA(w http.ResponseWriter, r *http.Request) {
	t, err := c.tracking.Status(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewTrackingResponse(t))
}

// AtRisk handles GET /v1/sla/at-risk.
func (c *Controller) AtRisk(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.tracking.AtRisk(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewTrackingResponses(items), page.Limit, func(t dto.TrackingResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: t.CreatedAt.UnixMilli(), ID: t.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}
