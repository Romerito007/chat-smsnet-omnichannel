// Package entity holds the auth aggregates persisted by the domain.
package entity

import "time"

// RefreshToken is a persisted, rotatable credential. Only the hash of the token
// is stored; the plaintext is returned to the client once at issue time.
type RefreshToken struct {
	ID        string
	TenantID  string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	UserAgent string
	IP        string
	CreatedAt time.Time
}

// Active reports whether the token can still be exchanged at time now.
func (t *RefreshToken) Active(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}
