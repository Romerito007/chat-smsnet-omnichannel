package auth

import (
	"net/http"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	authservice "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	iamcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/contracts"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/auth"
	iamdto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/iam"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// neutralMessage is the response for flows that must not reveal whether an email
// is registered (signup, forgot-password, resend-verification).
var neutralMessage = dto.MessageResponse{Message: "if the email is valid, you will receive an email shortly"}

// AccountController serves the account-lifecycle endpoints: signup, email
// verification, invitation acceptance, password reset and the authenticated
// profile (PATCH /me, change-password). The admin-only invite endpoint is
// mounted under /users/invite.
type AccountController struct {
	account *authservice.AccountService
	users   *iamservice.UserService
}

// NewAccountController builds the controller.
func NewAccountController(account *authservice.AccountService, users *iamservice.UserService) *AccountController {
	return &AccountController{account: account, users: users}
}

// Signup handles POST /v1/auth/signup. The response is neutral so an existing
// email is never leaked.
func (c *AccountController) Signup(w http.ResponseWriter, r *http.Request) {
	var req dto.SignupRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	if _, err := c.account.Signup(r.Context(), req.ToCommand()); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusAccepted, neutralMessage)
}

// VerifyEmail handles POST /v1/auth/verify-email.
func (c *AccountController) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req dto.VerifyEmailRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	if err := c.account.VerifyEmail(r.Context(), req.Token); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.MessageResponse{Message: "email verified"})
}

// ResendVerification handles POST /v1/auth/resend-verification (neutral).
func (c *AccountController) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req dto.EmailRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	if err := c.account.ResendVerification(r.Context(), req.Email); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusAccepted, neutralMessage)
}

// ForgotPassword handles POST /v1/auth/forgot-password (neutral).
func (c *AccountController) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req dto.EmailRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	if err := c.account.ForgotPassword(r.Context(), req.Email); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusAccepted, neutralMessage)
}

// ResetPassword handles POST /v1/auth/reset-password.
func (c *AccountController) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req dto.ResetPasswordRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	if err := c.account.ResetPassword(r.Context(), req.ToCommand()); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.MessageResponse{Message: "password updated"})
}

// AcceptInvite handles POST /v1/auth/accept-invite.
func (c *AccountController) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req dto.AcceptInviteRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	if err := c.account.AcceptInvite(r.Context(), req.ToCommand()); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.MessageResponse{Message: "account created"})
}

// Invite handles POST /v1/users/invite (admin; user.manage).
func (c *AccountController) Invite(w http.ResponseWriter, r *http.Request) {
	var req dto.InviteUserRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	inv, err := c.account.Invite(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.InviteResponse{ID: inv.ID, Email: inv.Email})
}

// UpdateMe handles PATCH /v1/me: the authenticated user edits their own profile.
func (c *AccountController) UpdateMe(w http.ResponseWriter, r *http.Request) {
	ac, ok := authz.FromContext(r.Context())
	if !ok {
		middleware.WriteError(w, r, apperror.Unauthorized("authentication required"))
		return
	}
	var req dto.UpdateProfileRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	user, err := c.users.UpdateProfile(r.Context(), ac.UserID, iamcontracts.UpdateProfile{
		Name:               req.Name,
		AvatarAttachmentID: req.AvatarAttachmentID,
	})
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, iamdto.NewUserResponse(user))
}

// ChangePassword handles POST /v1/me/change-password.
func (c *AccountController) ChangePassword(w http.ResponseWriter, r *http.Request) {
	ac, ok := authz.FromContext(r.Context())
	if !ok {
		middleware.WriteError(w, r, apperror.Unauthorized("authentication required"))
		return
	}
	var req dto.ChangePasswordRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	if err := c.users.ChangePassword(r.Context(), ac.UserID, req.CurrentPassword, req.NewPassword); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
