package models

import "time"

// AuditLog is the BSON document for an append-only audit entry.
type AuditLog struct {
	ID           string         `bson:"_id"`
	TenantID     string         `bson:"tenant_id"`
	ActorID      string         `bson:"actor_id,omitempty"`
	ActorType    string         `bson:"actor_type,omitempty"`
	Action       string         `bson:"action"`
	ResourceType string         `bson:"resource_type,omitempty"`
	ResourceID   string         `bson:"resource_id,omitempty"`
	IP           string         `bson:"ip,omitempty"`
	UserAgent    string         `bson:"user_agent,omitempty"`
	Data         map[string]any `bson:"data,omitempty"`
	CreatedAt    time.Time      `bson:"created_at"`
}
