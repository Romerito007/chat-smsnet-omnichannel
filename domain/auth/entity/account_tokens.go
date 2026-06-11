package entity

import "time"

// EmailVerificationToken is a single-use credential that activates a pending
// user. Only the hash is stored; the plaintext travels once in the emailed link.
type EmailVerificationToken struct {
	ID        string
	TenantID  string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// Usable reports whether the token can still be redeemed at time now.
func (t *EmailVerificationToken) Usable(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}

// PasswordResetToken is a single-use, short-lived credential authorizing a
// password reset. Only the hash is stored.
type PasswordResetToken struct {
	ID        string
	TenantID  string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// Usable reports whether the token can still be redeemed at time now.
func (t *PasswordResetToken) Usable(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}

// Invitation is a single-use credential that lets an invited person create their
// user (with the pre-assigned roles/sectors). Only the hash is stored.
type Invitation struct {
	ID         string
	TenantID   string
	Email      string
	RoleIDs    []string
	SectorIDs  []string
	TokenHash  string
	ExpiresAt  time.Time
	AcceptedAt *time.Time
	InvitedBy  string
	CreatedAt  time.Time
}

// Usable reports whether the invitation can still be accepted at time now.
func (i *Invitation) Usable(now time.Time) bool {
	return i.AcceptedAt == nil && now.Before(i.ExpiresAt)
}
