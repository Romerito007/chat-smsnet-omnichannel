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

// EmailVerificationToken activates a pending user. Only the hash is stored;
// expires_at carries a TTL index.
type EmailVerificationToken struct {
	ID        string     `bson:"_id"`
	TenantID  string     `bson:"tenant_id"`
	UserID    string     `bson:"user_id"`
	TokenHash string     `bson:"token_hash"`
	ExpiresAt time.Time  `bson:"expires_at"`
	UsedAt    *time.Time `bson:"used_at,omitempty"`
	CreatedAt time.Time  `bson:"created_at"`
}

// PasswordResetToken authorizes a single password reset. Only the hash is stored.
type PasswordResetToken struct {
	ID        string     `bson:"_id"`
	TenantID  string     `bson:"tenant_id"`
	UserID    string     `bson:"user_id"`
	TokenHash string     `bson:"token_hash"`
	ExpiresAt time.Time  `bson:"expires_at"`
	UsedAt    *time.Time `bson:"used_at,omitempty"`
	CreatedAt time.Time  `bson:"created_at"`
}

// Invitation is a single-use teammate invitation. Only the hash is stored.
type Invitation struct {
	ID         string     `bson:"_id"`
	TenantID   string     `bson:"tenant_id"`
	Email      string     `bson:"email"`
	RoleIDs    []string   `bson:"role_ids"`
	SectorIDs  []string   `bson:"sector_ids"`
	TokenHash  string     `bson:"token_hash"`
	ExpiresAt  time.Time  `bson:"expires_at"`
	AcceptedAt *time.Time `bson:"accepted_at,omitempty"`
	InvitedBy  string     `bson:"invited_by,omitempty"`
	CreatedAt  time.Time  `bson:"created_at"`
}
