package entity

import "time"

// Status is the user lifecycle state.
type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
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
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// IsActive reports whether the user may authenticate.
func (u *User) IsActive() bool { return u.Status == StatusActive }
