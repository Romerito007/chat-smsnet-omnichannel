package models

import "time"

// Tenant is the top-level isolation boundary. Every other document references
// its id via tenant_id.
type Tenant struct {
	ID          string         `bson:"_id"`
	Name        string         `bson:"name"`
	Status      string         `bson:"status"`
	ExternalRef string         `bson:"external_ref,omitempty"`
	Settings    map[string]any `bson:"settings,omitempty"`
	CreatedAt   time.Time      `bson:"created_at"`
	UpdatedAt   time.Time      `bson:"updated_at"`
}

// Role is a named permission bundle within a tenant.
type Role struct {
	Base        `bson:",inline"`
	Name        string   `bson:"name"`
	Permissions []string `bson:"permissions"`
	SectorScope string   `bson:"sector_scope"`
}

// User is an operator/agent account scoped to a tenant. PasswordHash is never
// projected to clients (mapping to DTOs drops it).
type User struct {
	Base                 `bson:",inline"`
	Name                 string         `bson:"name"`
	Email                string         `bson:"email"`
	PasswordHash         string         `bson:"password_hash"`
	Status               string         `bson:"status"`
	RoleIDs              []string       `bson:"role_ids"`
	SectorIDs            []string       `bson:"sector_ids"`
	MaxConcurrentChats   int            `bson:"max_concurrent_chats"`
	PresenceAvailability string         `bson:"presence_availability,omitempty"`
	AutoOffline          *bool          `bson:"auto_offline,omitempty"`
	AvatarAttachmentID   string         `bson:"avatar_attachment_id,omitempty"`
	Preferences          map[string]any `bson:"preferences,omitempty"`
}
