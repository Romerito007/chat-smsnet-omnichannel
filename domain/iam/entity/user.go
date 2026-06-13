package entity

import (
	"strings"
	"time"
)

// Status is the user lifecycle state.
type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
	// StatusPendingVerification is a self-signed-up or invited account that has
	// not yet confirmed its email. It cannot authenticate until activated.
	StatusPendingVerification Status = "pending_verification"
)

// User is an operator account (agent/supervisor/admin/owner) scoped to a tenant.
// PasswordHash is never serialized to clients.
type User struct {
	ID                 string
	TenantID           string
	Name               string
	Email              string
	PasswordHash       string
	Status             Status
	RoleIDs            []string
	SectorIDs          []string
	MaxConcurrentChats int
	// AvatarAttachmentID is the attachment (uploaded via the signed-URL flow) used
	// as the user's avatar. Optional.
	AvatarAttachmentID string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// IsActive reports whether the user may authenticate.
func (u *User) IsActive() bool { return u.Status == StatusActive }

// NormalizeSectorIDs returns the sector ids trimmed, with empty entries and
// duplicates removed, never nil. "Sem setor" is always the empty slice — never
// nil, never [""] — so a sector membership check can never falsely match a real
// sector id against a junk empty entry.
func NormalizeSectorIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
