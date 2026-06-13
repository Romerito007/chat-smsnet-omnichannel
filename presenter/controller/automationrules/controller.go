// Package automationrules holds the HTTP controller for automation-rules CRUD.
package automationrules

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	arservice "github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/automationrules"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves automation-rules CRUD.
type Controller struct {
	rules *arservice.RuleService
}

// NewController builds the controller.
func NewController(rules *arservice.RuleService) *Controller {
	return &Controller{rules: rules}
}

// List handles GET /v1/automation-rules.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	rs, err := c.rules.List(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewRuleListResponse(rs))
}

// Create handles POST /v1/automation-rules.
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateRuleRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	rule, err := c.rules.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewRuleResponse(rule))
}

// Get handles GET /v1/automation-rules/{id}.
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	rule, err := c.rules.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewRuleResponse(rule))
}

// Update handles PATCH /v1/automation-rules/{id}.
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateRuleRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	rule, err := c.rules.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewRuleResponse(rule))
}

// Delete handles DELETE /v1/automation-rules/{id}.
func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.rules.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Logs handles GET /v1/automation-rules/{id}/logs (keyset pagination).
func (c *Controller) Logs(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.rules.Logs(r.Context(), chi.URLParam(r, "id"), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewEvaluationLogResponses(items), page.Limit, func(l dto.EvaluationLogResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: l.CreatedAt.UnixMilli(), ID: l.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}
