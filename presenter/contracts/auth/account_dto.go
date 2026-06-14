package auth

import authcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/contracts"

// SignupRequest is the body of POST /v1/auth/signup.
type SignupRequest struct {
	CompanyName string `json:"company_name"`
	OwnerName   string `json:"owner_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

// ToCommand maps to the service command.
func (r SignupRequest) ToCommand() authcontracts.SignupCommand {
	return authcontracts.SignupCommand{
		CompanyName: r.CompanyName,
		OwnerName:   r.OwnerName,
		Email:       r.Email,
		Password:    r.Password,
	}
}

// VerifyEmailRequest is the body of POST /v1/auth/verify-email.
type VerifyEmailRequest struct {
	Token string `json:"token"`
}

// EmailRequest is the body of the email-only endpoints (resend-verification,
// forgot-password).
type EmailRequest struct {
	Email string `json:"email"`
}

// ResetPasswordRequest is the body of POST /v1/auth/reset-password.
type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// ToCommand maps to the service command.
func (r ResetPasswordRequest) ToCommand() authcontracts.ResetPasswordCommand {
	return authcontracts.ResetPasswordCommand{Token: r.Token, NewPassword: r.NewPassword}
}

// AcceptInviteRequest is the body of POST /v1/auth/accept-invite.
type AcceptInviteRequest struct {
	Token    string `json:"token"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

// ToCommand maps to the service command.
func (r AcceptInviteRequest) ToCommand() authcontracts.AcceptInviteCommand {
	return authcontracts.AcceptInviteCommand{Token: r.Token, Name: r.Name, Password: r.Password}
}

// InviteUserRequest is the body of POST /v1/users/invite.
type InviteUserRequest struct {
	Email     string   `json:"email"`
	RoleIDs   []string `json:"role_ids"`
	SectorIDs []string `json:"sector_ids"`
}

// ToCommand maps to the service command.
func (r InviteUserRequest) ToCommand() authcontracts.InviteCommand {
	return authcontracts.InviteCommand{Email: r.Email, RoleIDs: r.RoleIDs, SectorIDs: r.SectorIDs}
}

// UpdateProfileRequest is the body of PATCH /v1/me.
type UpdateProfileRequest struct {
	Name               *string `json:"name"`
	AvatarAttachmentID *string `json:"avatar_attachment_id"`
	// Preferences, when present, full-replaces the user's stored UI preferences
	// (theme, audio_alerts, browser_push, …). Free/nested object; omit to leave
	// unchanged. Only theme and audio_alerts.play_for are validated server-side.
	Preferences *map[string]any `json:"preferences"`
}

// ChangePasswordRequest is the body of POST /v1/me/change-password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// MessageResponse is a neutral acknowledgement returned by the account flows that
// must not leak whether an email exists.
type MessageResponse struct {
	Message string `json:"message"`
}

// InviteResponse acknowledges a created invitation (no token — it is emailed).
type InviteResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}
