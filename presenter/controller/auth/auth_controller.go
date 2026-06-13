// Package auth holds the HTTP controllers for the authentication endpoints.
package auth

import (
	"net/http"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	authcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/contracts"
	authservice "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/auth"
	iamdto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/iam"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves login, refresh, logout and the authenticated /me endpoint.
type Controller struct {
	auth  *authservice.Service
	users *iamservice.UserService
}

// NewController builds the controller.
func NewController(auth *authservice.Service, users *iamservice.UserService) *Controller {
	return &Controller{auth: auth, users: users}
}

// Login authenticates a user and returns a token pair.
func (c *Controller) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	pair, err := c.auth.Login(r.Context(), authcontracts.LoginCommand{
		Email:     req.Email,
		Password:  req.Password,
		UserAgent: r.UserAgent(),
		IP:        middleware.ClientIP(r),
	})
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewTokenResponse(pair))
}

// Refresh rotates a refresh token and returns a new pair.
func (c *Controller) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	pair, err := c.auth.Refresh(r.Context(), authcontracts.RefreshCommand{
		RefreshToken: req.RefreshToken,
		UserAgent:    r.UserAgent(),
		IP:           middleware.ClientIP(r),
	})
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewTokenResponse(pair))
}

// Logout revokes a refresh token. Idempotent.
func (c *Controller) Logout(w http.ResponseWriter, r *http.Request) {
	var req dto.LogoutRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	if err := c.auth.Logout(r.Context(), authcontracts.LogoutCommand{RefreshToken: req.RefreshToken}); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Me returns the authenticated user with their effective permissions and scope.
func (c *Controller) Me(w http.ResponseWriter, r *http.Request) {
	ac, ok := authz.FromContext(r.Context())
	if !ok {
		middleware.WriteError(w, r, apperror.Unauthorized("authentication required"))
		return
	}
	user, err := c.users.Get(r.Context(), ac.UserID)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	avatars, _ := c.users.AvatarURLs(r.Context(), []string{user.AvatarAttachmentID})
	resp := dto.MeResponse{
		User:        iamdto.NewUserResponseWithAvatar(user, avatars),
		Permissions: permList(ac),
		SectorScope: string(ac.SectorScope),
		SectorIDs:   ac.SectorIDs,
	}
	middleware.WriteJSON(w, http.StatusOK, resp)
}

func permList(ac authz.AuthContext) []string {
	perms := ac.PermissionList()
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = string(p)
	}
	return out
}
