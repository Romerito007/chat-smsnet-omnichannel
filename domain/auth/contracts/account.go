package contracts

// SignupCommand is the self-service company signup input. It provisions a tenant
// plus its owner user (pending email verification).
type SignupCommand struct {
	CompanyName string
	OwnerName   string
	Email       string
	Password    string
}

// SignupResult reports the outcome of a signup. Created is false when the email
// was already registered (the response is kept neutral so existence is not
// leaked); TenantID/UserID are set only on a genuine creation.
type SignupResult struct {
	Created  bool
	TenantID string
	UserID   string
}

// InviteCommand is the admin "invite a teammate" input. The tenant and inviter
// come from the authenticated context.
type InviteCommand struct {
	Email     string
	RoleIDs   []string
	SectorIDs []string
}

// AcceptInviteCommand redeems an invitation token, creating the invited user.
type AcceptInviteCommand struct {
	Token    string
	Name     string
	Password string
}

// ResetPasswordCommand redeems a password-reset token.
type ResetPasswordCommand struct {
	Token       string
	NewPassword string
}
