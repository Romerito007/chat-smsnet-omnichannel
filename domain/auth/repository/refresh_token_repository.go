// Package repository declares the auth persistence contract. The implementation
// lives in infra/database/mongodb/repositories/auth.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth/entity"
)

// RefreshTokenRepository persists refresh tokens.
//
// FindByHash is intentionally NOT tenant-scoped: at refresh time the request is
// unauthenticated, and the token record itself carries the authoritative tenant.
// The token hash is derived from 256 bits of randomness, so it is globally
// unique.
type RefreshTokenRepository interface {
	Create(ctx context.Context, t *entity.RefreshToken) error
	FindByHash(ctx context.Context, tokenHash string) (*entity.RefreshToken, error)
	Revoke(ctx context.Context, id string) error
	RevokeAllForUser(ctx context.Context, tenantID, userID string) error
}
