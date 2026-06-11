package contracts

import "context"

// AccountEmail is a transactional account email to render and deliver. Link is
// the action URL (carrying the single-use token); Company/Name personalize the
// template. The token itself is never logged.
type AccountEmail struct {
	To      string
	Name    string
	Company string
	Link    string
}

// Mailer delivers the account-lifecycle emails (verification, invitation,
// password reset and reset confirmation). Implemented by infra/email over SMTP
// with HTML templates. The auth domain depends only on this port.
type Mailer interface {
	// SendVerification delivers the "confirm your email" message after signup or
	// a resend request.
	SendVerification(ctx context.Context, msg AccountEmail) error
	// SendInvite delivers a teammate invitation with the accept link.
	SendInvite(ctx context.Context, msg AccountEmail) error
	// SendPasswordReset delivers the password-reset link.
	SendPasswordReset(ctx context.Context, msg AccountEmail) error
	// SendPasswordResetDone confirms a completed password reset (no link/token).
	SendPasswordResetDone(ctx context.Context, msg AccountEmail) error
}
