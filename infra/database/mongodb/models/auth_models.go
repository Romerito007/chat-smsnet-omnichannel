package models

import "time"

// RefreshToken is the persisted, rotatable refresh credential. Only the hash is
// stored. expires_at carries a TTL index for automatic cleanup.
type RefreshToken struct {
	ID        string     `bson:"_id"`
	TenantID  string     `bson:"tenant_id"`
	UserID    string     `bson:"user_id"`
	TokenHash string     `bson:"token_hash"`
	ExpiresAt time.Time  `bson:"expires_at"`
	RevokedAt *time.Time `bson:"revoked_at,omitempty"`
	UserAgent string     `bson:"user_agent,omitempty"`
	IP        string     `bson:"ip,omitempty"`
	CreatedAt time.Time  `bson:"created_at"`
}
