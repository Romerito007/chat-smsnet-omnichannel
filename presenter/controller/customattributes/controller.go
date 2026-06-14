// Package customattributes holds the HTTP controller for custom-attribute
// definition CRUD.
package customattributes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	caentity "github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/entity"
	caservice "github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/customattributes"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves custom-attribute definition CRUD.
type Controller struct {
	svc *caservice.Service
}

// NewController builds the controller.
func NewController(svc *caservice.Service) *Controller {
	return &Controller{svc: svc}
}

// List handles GET /v1/custom-attributes?applies_to=contact|conversation.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	appliesTo := caentity.AppliesTo(r.URL.Query().Get("applies_to"))
	items, err := c.svc.List(r.Context(), appliesTo, page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewDefinitionResponses(items), page.Limit, func(d dto.DefinitionResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: d.CreatedAt.UnixMilli(), ID: d.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// Create handles POST /v1/custom-attributes.
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateDefinitionRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	d, err := c.svc.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewDefinitionResponse(d))
}

// Get handles GET /v1/custom-attributes/{id}.
func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	d, err := c.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewDefinitionResponse(d))
}

// Update handles PATCH /v1/custom-attributes/{id}.
func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateDefinitionRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	d, err := c.svc.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewDefinitionResponse(d))
}

// Delete handles DELETE /v1/custom-attributes/{id}.
func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.svc.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
