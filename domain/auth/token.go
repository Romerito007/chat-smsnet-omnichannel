// Package auth declares the authentication ports (token manager) and is the home
// of the auth aggregates (refresh tokens), contracts and service. It depends on
// authz for the permission/scope vocabulary embedded in access tokens.
package auth

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
)

// AccessClaims is the authenticated identity carried by a verified access token.
// Permissions and sector scope are embedded so the AuthContext middleware needs
// no database lookup per request (tokens are short-lived).
type AccessClaims struct {
	TenantID    string
	UserID      string
	Permissions []authz.Permission
	SectorIDs   []string
	SectorScope authz.SectorScope
	ExpiresAt   time.Time
}

// TokenManager issues and verifies access tokens and mints refresh tokens. The
// implementation (infra/security) owns the signing algorithm and TTLs.
type TokenManager interface {
	// IssueAccess signs an access token for the given identity, returning the
	// token and its expiry. The manager sets the expiry from its configured TTL.
	IssueAccess(claims AccessClaims) (token string, expiresAt time.Time, err error)
	// VerifyAccess validates the token signature and expiry and returns its claims.
	VerifyAccess(token string) (AccessClaims, error)
	// GenerateRefresh mints a new opaque refresh token (plaintext) and its expiry.
	GenerateRefresh() (plaintext string, expiresAt time.Time, err error)
	// HashRefresh derives the storage/lookup hash for a refresh token plaintext.
	HashRefresh(plaintext string) string
}
