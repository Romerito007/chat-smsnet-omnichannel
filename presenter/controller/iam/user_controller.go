// Package iam holds the HTTP controllers for the IAM endpoints (users, roles).
package iam

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/iam"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// userAvatarURLs batch-resolves the signed avatar URLs for a set of users (keyed
// by avatar attachment id). Shared by the user, agents and /me presenters so
// agent avatars render in the inbox without a per-item request. Best-effort.
func userAvatarURLs(r *http.Request, svc *iamservice.UserService, users []*iamentity.User) map[string]string {
	ids := make([]string, 0, len(users))
	for _, u := range users {
		if u.AvatarAttachmentID != "" {
			ids = append(ids, u.AvatarAttachmentID)
		}
	}
	urls, _ := svc.AvatarURLs(r.Context(), ids)
	return urls
}

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
	middleware.WriteJSON(w, http.StatusCreated, dto.NewUserResponseWithAvatar(user, userAvatarURLs(r, c.users, []*iamentity.User{user})))
}

// List handles GET /v1/users.
func (c *UserController) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	users, err := c.users.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	avatars := userAvatarURLs(r, c.users, users)
	resp := shared.NewPage(dto.NewUserResponsesWithAvatars(users, avatars), page.Limit, func(u dto.UserResponse) shared.Cursor {
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
	avatars := userAvatarURLs(r, c.users, []*iamentity.User{user})
	middleware.WriteJSON(w, http.StatusOK, dto.NewUserResponseWithAvatar(user, avatars))
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
	middleware.WriteJSON(w, http.StatusOK, dto.NewUserResponseWithAvatar(user, userAvatarURLs(r, c.users, []*iamentity.User{user})))
}

// Delete handles DELETE /v1/users/{id}.
func (c *UserController) Delete(w http.ResponseWriter, r *http.Request) {
	if err := c.users.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
