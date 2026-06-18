// Package audit holds the response DTOs for the audit-log endpoint.
package audit

import (
	"time"

	aentity "github.com/romerito007/chat-smsnet-omnichannel/domain/audit/entity"
)

// AuditLogResponse is the public view of an audit log entry.
type AuditLogResponse struct {
	ID      string `json:"id"`
	ActorID string `json:"actor_id,omitempty"`
	// ActorName is read-only/derived: the actor's display name, resolved in batch so
	// the log renders the name instead of a raw id; empty for non-user actors
	// (system/platform) or when unresolved.
	ActorName    string         `json:"actor_name,omitempty"`
	ActorType    string         `json:"actor_type,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type,omitempty"`
	ResourceID   string         `json:"resource_id,omitempty"`
	IP           string         `json:"ip,omitempty"`
	UserAgent    string         `json:"user_agent,omitempty"`
	Data         map[string]any `json:"data,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// NewAuditLogResponse maps an entity.
func NewAuditLogResponse(l *aentity.AuditLog) AuditLogResponse {
	return AuditLogResponse{
		ID:           l.ID,
		ActorID:      l.ActorID,
		ActorType:    l.ActorType,
		Action:       l.Action,
		ResourceType: l.ResourceType,
		ResourceID:   l.ResourceID,
		IP:           l.IP,
		UserAgent:    l.UserAgent,
		Data:         l.Data,
		CreatedAt:    l.CreatedAt,
	}
}

// NewAuditLogResponses maps a slice.
func NewAuditLogResponses(items []*aentity.AuditLog) []AuditLogResponse {
	out := make([]AuditLogResponse, 0, len(items))
	for _, l := range items {
		out = append(out, NewAuditLogResponse(l))
	}
	return out
}
