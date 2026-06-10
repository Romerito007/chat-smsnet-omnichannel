// Package channels holds the request/response DTOs for the channel integration
// management endpoints and the inbound endpoint.
package channels

import (
	"time"

	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// ── integration management ───────────────────────────────────────────────────

// CreateIntegrationRequest is the body of POST /v1/channels.
type CreateIntegrationRequest struct {
	Channel           string `json:"channel"`
	Name              string `json:"name"`
	AutomationEnabled bool   `json:"automation_enabled"`
	DefaultQueueID    string `json:"default_queue_id"`
}

// ToCommand maps to the service command.
func (r CreateIntegrationRequest) ToCommand() chcontracts.CreateIntegration {
	return chcontracts.CreateIntegration{
		Channel:           r.Channel,
		Name:              r.Name,
		AutomationEnabled: r.AutomationEnabled,
		DefaultQueueID:    r.DefaultQueueID,
	}
}

// IntegrationResponse is the public representation of an integration. The secret
// is only included on creation (so the provider can be configured) and omitted
// elsewhere.
type IntegrationResponse struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	Channel           string    `json:"channel"`
	Name              string    `json:"name,omitempty"`
	IntegrationKey    string    `json:"integration_key"`
	Secret            string    `json:"secret,omitempty"`
	Enabled           bool      `json:"enabled"`
	AutomationEnabled bool      `json:"automation_enabled"`
	DefaultQueueID    string    `json:"default_queue_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// NewIntegrationResponse maps an integration; includeSecret controls secret
// exposure.
func NewIntegrationResponse(i *chentity.Integration, includeSecret bool) IntegrationResponse {
	resp := IntegrationResponse{
		ID:                i.ID,
		TenantID:          i.TenantID,
		Channel:           i.Channel,
		Name:              i.Name,
		IntegrationKey:    i.IntegrationKey,
		Enabled:           i.Enabled,
		AutomationEnabled: i.AutomationEnabled,
		DefaultQueueID:    i.DefaultQueueID,
		CreatedAt:         i.CreatedAt,
		UpdatedAt:         i.UpdatedAt,
	}
	if includeSecret {
		resp.Secret = i.Secret
	}
	return resp
}

// NewIntegrationResponses maps a slice (without secrets).
func NewIntegrationResponses(items []*chentity.Integration) []IntegrationResponse {
	out := make([]IntegrationResponse, len(items))
	for i, it := range items {
		out[i] = NewIntegrationResponse(it, false)
	}
	return out
}

// ── inbound ──────────────────────────────────────────────────────────────────

// AttachmentItem mirrors an attachment on the wire.
type AttachmentItem struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
}

// InboundRequest is the body of POST /v1/inbound/channel/{channel}/messages.
type InboundRequest struct {
	TenantKey         string           `json:"tenant_key"`
	IntegrationKey    string           `json:"integration_key"`
	ExternalMessageID string           `json:"external_message_id"`
	ExternalContactID string           `json:"external_contact_id"`
	ContactName       string           `json:"contact_name"`
	ContactPhone      string           `json:"contact_phone"`
	ContactDocument   string           `json:"contact_document"`
	Channel           string           `json:"channel"`
	Text              string           `json:"text"`
	Attachments       []AttachmentItem `json:"attachments"`
	Metadata          map[string]any   `json:"metadata"`
	Timestamp         int64            `json:"timestamp"`
}

// ToMessage maps the request to the domain inbound message, using the path
// channel as authoritative.
func (r InboundRequest) ToMessage(channel string) chcontracts.InboundMessage {
	atts := make([]conventity.Attachment, len(r.Attachments))
	for i, a := range r.Attachments {
		atts[i] = conventity.Attachment{ID: a.ID, URL: a.URL, ContentType: a.ContentType, Filename: a.Filename, Size: a.Size}
	}
	return chcontracts.InboundMessage{
		TenantKey:         r.TenantKey,
		IntegrationKey:    r.IntegrationKey,
		ExternalMessageID: r.ExternalMessageID,
		ExternalContactID: r.ExternalContactID,
		ContactName:       r.ContactName,
		ContactPhone:      r.ContactPhone,
		ContactDocument:   r.ContactDocument,
		Channel:           channel,
		Text:              r.Text,
		Attachments:       atts,
		Metadata:          r.Metadata,
		Timestamp:         r.Timestamp,
	}
}
