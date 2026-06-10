// Package iam holds the HTTP controllers for the IAM endpoints (users, roles).
package iam

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/iam"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// UserController serves CRUD for users.
type UserController struct {
	users *iamservice.UserService
}

// NewUserController builds the controller.
func NewUserController(users *iamservice.UserService) *UserController {
	return &UserController{users: users}
}

// Create handles POST /v1/users.
func (c *UserController) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateUserRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	user, err := c.users.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewUserResponse(user))
}

// List handles GET /v1/users.
func (c *UserController) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	users, err := c.users.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewUserResponses(users), page.Limit, func(u dto.UserResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: u.CreatedAt.UnixMilli(), ID: u.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}

// Get handles GET /v1/users/{id}.
func (c *UserController) Get(w http.ResponseWriter, r *http.Request) {
	user, err := c.users.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewUserResponse(user))
}

// Update handles PATCH /v1/users/{id}.
func (c *UserController) Update(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateUserRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	user, err := c.users.Update(r.Context(), chi.URLParam(r, "id"), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewUserResponse(user))
}

// Delete handles DELETE /v1/users/{id}.
func (c *UserController) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.users.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
