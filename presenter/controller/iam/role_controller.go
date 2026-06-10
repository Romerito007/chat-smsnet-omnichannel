package iam

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/iam"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// RoleController serves CRUD for roles.
type RoleController struct {
	roles *iamservice.RoleService
}

// NewRoleController builds the controller.
func NewRoleController(roles *iamservice.RoleService) *RoleController {
	return &RoleController{roles: roles}
}

// Create handles POST /v1/roles.
func (c *RoleController) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateRoleRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	role, err := c.roles.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewRoleResponse(role))
}

// List handles GET /v1/roles.
func (c *RoleController) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	roles, err := c.roles.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewRoleResponses(roles), page.Limit, func(role dto.RoleResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: role.CreatedAt.UnixMilli(), ID: role.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// Get handles GET /v1/roles/{id}.
func (c *RoleController) Get(w http.ResponseWriter, r *http.Request) {
	role, err := c.roles.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewRoleResponse(role))
}

// Update handles PATCH /v1/roles/{id}.
func (c *RoleController) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateRoleRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	role, err := c.roles.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewRoleResponse(role))
}

// Delete handles DELETE /v1/roles/{id}.
func (c *RoleController) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.roles.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
