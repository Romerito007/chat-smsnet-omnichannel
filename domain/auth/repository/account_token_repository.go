package repository

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth/entity"
)

// The account-token repositories below resolve a record by its token hash. Like
// refresh tokens, that lookup is intentionally NOT tenant-scoped: the redeeming
// request is unauthenticated and the record carries the authoritative tenant.

// EmailVerificationTokenRepository persists email-verification tokens.
type EmailVerificationTokenRepository interface {
	Create(ctx context.Context, t *entity.EmailVerificationToken) error
	FindByHash(ctx context.Context, tokenHash string) (*entity.EmailVerificationToken, error)
	MarkUsed(ctx context.Context, id string, usedAt time.Time) error
	// InvalidateForUser marks every outstanding token for a user used, so a resend
	// supersedes prior links. Tenant-scoped via context.
	InvalidateForUser(ctx context.Context, userID string, usedAt time.Time) error
}

// PasswordResetTokenRepository persists password-reset tokens.
type PasswordResetTokenRepository interface {
	Create(ctx context.Context, t *entity.PasswordResetToken) error
	FindByHash(ctx context.Context, tokenHash string) (*entity.PasswordResetToken, error)
	MarkUsed(ctx context.Context, id string, usedAt time.Time) error
	InvalidateForUser(ctx context.Context, userID string, usedAt time.Time) error
}

// InvitationRepository persists user invitations.
type InvitationRepository interface {
	Create(ctx context.Context, i *entity.Invitation) error
	FindByHash(ctx context.Context, tokenHash string) (*entity.Invitation, error)
	// FindPendingByEmail returns an outstanding invitation for an email within the
	// current tenant, or a not_found error.
	FindPendingByEmail(ctx context.Context, email string) (*entity.Invitation, error)
	MarkAccepted(ctx context.Context, id string, acceptedAt time.Time) error
}
