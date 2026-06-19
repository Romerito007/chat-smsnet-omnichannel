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
	// PresenceAvailability is the agent's DURABLE manual availability
	// (online|away|offline) — a persistent override that sticks to the account
	// across logout/reconnect/tab/machine and the Redis TTL. Empty = never set →
	// defaults to online. Changed only by the agent (POST presence/status).
	PresenceAvailability string
	// AutoOffline governs what happens when the agent's LAST live socket drops while
	// availability is online: true → effective offline; false → stays online. nil =
	// never set → defaults to true. It never affects a manual away/offline.
	AutoOffline *bool
	// AvatarAttachmentID is the attachment (uploaded via the signed-URL flow) used
	// as the user's avatar. Optional.
	AvatarAttachmentID string
	// Preferences is the umbrella object for ALL of the user's UI preferences
	// (theme, audio alerts, browser push, …), so they follow the user across
	// devices instead of living in the browser's localStorage. Free/nested by
	// design: the backend only stores and returns it, validating just the few
	// enum-constrained fields (theme, audio_alerts.play_for). Read/written through
	// GET/PATCH /v1/me. Email-server preferences stay in the notifications domain.
	Preferences map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsActive reports whether the user may authenticate.
func (u *User) IsActive() bool { return u.Status == StatusActive }

// AvailabilityOr returns the agent's durable manual availability, defaulting to
// "online" when it was never set.
func (u *User) AvailabilityOr() string {
	if strings.TrimSpace(u.PresenceAvailability) == "" {
		return "online"
	}
	return u.PresenceAvailability
}

// AutoOfflineOr returns the auto-offline toggle, defaulting to true when never set.
func (u *User) AutoOfflineOr() bool {
	if u.AutoOffline == nil {
		return true
	}
	return *u.AutoOffline
}

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
