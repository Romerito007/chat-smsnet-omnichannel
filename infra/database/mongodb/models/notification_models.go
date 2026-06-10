package models

import "time"

// Notification is the BSON document for an in-app notification.
type Notification struct {
	ID        string     `bson:"_id"`
	TenantID  string     `bson:"tenant_id"`
	UserID    string     `bson:"user_id"`
	Type      string     `bson:"type"`
	Title     string     `bson:"title"`
	Body      string     `bson:"body,omitempty"`
	Link      string     `bson:"link,omitempty"`
	Read      bool       `bson:"read"`
	CreatedAt time.Time  `bson:"created_at"`
	ReadAt    *time.Time `bson:"read_at,omitempty"`
}

// NotificationPreferences is the BSON document for a user's per-type email
// preferences.
type NotificationPreferences struct {
	ID          string          `bson:"_id"` // tenant_id:user_id
	TenantID    string          `bson:"tenant_id"`
	UserID      string          `bson:"user_id"`
	EmailByType map[string]bool `bson:"email_by_type,omitempty"`
	UpdatedAt   time.Time       `bson:"updated_at"`
}
