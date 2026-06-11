// Package audit holds the response DTOs for the audit-log endpoint.
package audit

import (
	"time"

	aentity "github.com/romerito007/chat-smsnet-omnichannel/domain/audit/entity"
)

// AuditLogResponse is the public view of an audit log entry.
type AuditLogResponse struct {
	ID           string         `json:"id"`
	ActorID      string         `json:"actor_id,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type,omitempty"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// NewAuditLogResponse maps an entity.
func NewAuditLogResponse(l *aentity.AuditLog) AuditLogResponse {
	return AuditLogResponse{
		ID:           l.ID,
		ActorID:      l.ActorID,
		Action:       l.Action,
		ResourceType: l.ResourceType,
		ResourceID:   l.ResourceID,
		Metadata:     l.Metadata,
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
