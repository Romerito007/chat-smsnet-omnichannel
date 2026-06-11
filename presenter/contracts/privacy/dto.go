// Package privacy holds the request/response DTOs for the privacy (LGPD)
// endpoints.
package privacy

import (
	"time"

	pcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/contracts"
	pentity "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/entity"
)

// ExportResponse is the public view of a data-export request.
type ExportResponse struct {
	ID          string     `json:"id"`
	ContactID   string     `json:"contact_id"`
	Status      string     `json:"status"`
	DownloadURL string     `json:"download_url,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// NewExportResponse maps an export request entity.
func NewExportResponse(e *pentity.ExportRequest) ExportResponse {
	resp := ExportResponse{
		ID:          e.ID,
		ContactID:   e.ContactID,
		Status:      string(e.Status),
		DownloadURL: e.DownloadURL,
		Error:       e.Error,
		CreatedAt:   e.CreatedAt,
		CompletedAt: e.CompletedAt,
	}
	if !e.ExpiresAt.IsZero() {
		resp.ExpiresAt = &e.ExpiresAt
	}
	return resp
}

// RetentionResponse is the public view of a retention policy.
type RetentionResponse struct {
	MessagesDays            int        `json:"messages_days"`
	ClosedConversationsDays int        `json:"closed_conversations_days"`
	TechnicalLogsDays       int        `json:"technical_logs_days"`
	AuditLogsDays           int        `json:"audit_logs_days"`
	NotificationsDays       int        `json:"notifications_days"`
	UpdatedAt               *time.Time `json:"updated_at,omitempty"`
}

// NewRetentionResponse maps a retention policy entity.
func NewRetentionResponse(p *pentity.RetentionPolicy) RetentionResponse {
	resp := RetentionResponse{
		MessagesDays:            p.MessagesDays,
		ClosedConversationsDays: p.ClosedConversationsDays,
		TechnicalLogsDays:       p.TechnicalLogsDays,
		AuditLogsDays:           p.AuditLogsDays,
		NotificationsDays:       p.NotificationsDays,
	}
	if !p.UpdatedAt.IsZero() {
		resp.UpdatedAt = &p.UpdatedAt
	}
	return resp
}

// UpdateRetentionRequest is the body of PATCH /v1/privacy/retention. Pointer
// fields allow partial updates.
type UpdateRetentionRequest struct {
	MessagesDays            *int `json:"messages_days"`
	ClosedConversationsDays *int `json:"closed_conversations_days"`
	TechnicalLogsDays       *int `json:"technical_logs_days"`
	AuditLogsDays           *int `json:"audit_logs_days"`
	NotificationsDays       *int `json:"notifications_days"`
}

// ToCommand maps to the service command.
func (r UpdateRetentionRequest) ToCommand() pcontracts.UpdateRetention {
	return pcontracts.UpdateRetention{
		MessagesDays:            r.MessagesDays,
		ClosedConversationsDays: r.ClosedConversationsDays,
		TechnicalLogsDays:       r.TechnicalLogsDays,
		AuditLogsDays:           r.AuditLogsDays,
		NotificationsDays:       r.NotificationsDays,
	}
}
