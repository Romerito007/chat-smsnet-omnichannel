// Package channels holds the request/response DTOs for channel connection
// management, the inbound endpoint and delivery receipts.
package channels

import (
	"time"

	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// ── connection management ────────────────────────────────────────────────────

// CreateConnectionRequest is the body of POST /v1/channels. For the API channel
// (type=api) the outbound webhook is configured via outbound_url/outbound_secret;
// these are accepted as aliases of base_url/secret. The inbound_token is always
// generated server-side.
type CreateConnectionRequest struct {
	Type            string         `json:"type"`
	Name            string         `json:"name"`
	BaseURL         string         `json:"base_url"`
	OutboundURL     string         `json:"outbound_url"`
	AuthType        string         `json:"auth_type"`
	Secret          string         `json:"secret"`
	OutboundSecret  string         `json:"outbound_secret"`
	DefaultSectorID string         `json:"default_sector_id"`
	BusinessHours   map[string]any `json:"business_hours"`
}

// ToCommand maps to the service command, preferring the API-channel field names
// (outbound_url/outbound_secret) when present.
func (r CreateConnectionRequest) ToCommand() chcontracts.CreateConnection {
	baseURL := r.BaseURL
	if r.OutboundURL != "" {
		baseURL = r.OutboundURL
	}
	secret := r.Secret
	if r.OutboundSecret != "" {
		secret = r.OutboundSecret
	}
	return chcontracts.CreateConnection{
		Type:            chentity.Type(r.Type),
		Name:            r.Name,
		BaseURL:         baseURL,
		AuthType:        chentity.AuthType(r.AuthType),
		Secret:          secret,
		DefaultSectorID: r.DefaultSectorID,
		BusinessHours:   r.BusinessHours,
	}
}

// UpdateConnectionRequest is the body of PATCH /v1/channels/{id}.
type UpdateConnectionRequest struct {
	Name            *string         `json:"name"`
	Status          *string         `json:"status"`
	BaseURL         *string         `json:"base_url"`
	OutboundURL     *string         `json:"outbound_url"`
	AuthType        *string         `json:"auth_type"`
	Secret          *string         `json:"secret"`
	OutboundSecret  *string         `json:"outbound_secret"`
	DefaultSectorID *string         `json:"default_sector_id"`
	BusinessHours   *map[string]any `json:"business_hours"`
	Enabled         *bool           `json:"enabled"`
}

// ToCommand maps to the service command, accepting the API-channel field names
// (outbound_url/outbound_secret) as aliases of base_url/secret.
func (r UpdateConnectionRequest) ToCommand() chcontracts.UpdateConnection {
	baseURL := r.BaseURL
	if r.OutboundURL != nil {
		baseURL = r.OutboundURL
	}
	secret := r.Secret
	if r.OutboundSecret != nil {
		secret = r.OutboundSecret
	}
	cmd := chcontracts.UpdateConnection{
		Name:            r.Name,
		BaseURL:         baseURL,
		Secret:          secret,
		DefaultSectorID: r.DefaultSectorID,
		BusinessHours:   r.BusinessHours,
		Enabled:         r.Enabled,
	}
	if r.Status != nil {
		st := chentity.Status(*r.Status)
		cmd.Status = &st
	}
	if r.AuthType != nil {
		at := chentity.AuthType(*r.AuthType)
		cmd.AuthType = &at
	}
	return cmd
}

// ConnectionResponse is the public representation of a connection. Neither the
// outbound secret nor the inbound token is ever returned here (only whether each
// is set); both are revealed only once, on creation, via CreatedConnectionResponse.
type ConnectionResponse struct {
	ID              string         `json:"id"`
	TenantID        string         `json:"tenant_id"`
	Type            string         `json:"type"`
	Name            string         `json:"name,omitempty"`
	Status          string         `json:"status"`
	BaseURL         string         `json:"base_url,omitempty"`
	AuthType        string         `json:"auth_type,omitempty"`
	HasSecret       bool           `json:"has_secret"`
	HasInboundToken bool           `json:"has_inbound_token"`
	DefaultSectorID string         `json:"default_sector_id,omitempty"`
	BusinessHours   map[string]any `json:"business_hours,omitempty"`
	Enabled         bool           `json:"enabled"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// NewConnectionResponse maps a connection, masking both the secret and token.
func NewConnectionResponse(c *chentity.ChannelConnection) ConnectionResponse {
	return ConnectionResponse{
		ID:              c.ID,
		TenantID:        c.TenantID,
		Type:            string(c.Type),
		Name:            c.Name,
		Status:          string(c.Status),
		BaseURL:         c.BaseURL,
		AuthType:        string(c.AuthType),
		HasSecret:       c.Secret != "",
		HasInboundToken: c.InboundTokenHash != "",
		DefaultSectorID: c.DefaultSectorID,
		BusinessHours:   c.BusinessHours,
		Enabled:         c.Enabled,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

// NewConnectionResponses maps a slice.
func NewConnectionResponses(items []*chentity.ChannelConnection) []ConnectionResponse {
	out := make([]ConnectionResponse, len(items))
	for i, c := range items {
		out[i] = NewConnectionResponse(c)
	}
	return out
}

// CreatedConnectionResponse is returned once, on creation. It is the only place
// the inbound_token and outbound_secret are revealed; afterwards they are masked.
type CreatedConnectionResponse struct {
	ConnectionResponse
	InboundToken   string `json:"inbound_token"`
	OutboundSecret string `json:"outbound_secret,omitempty"`
}

// NewCreatedConnectionResponse maps a freshly created connection, revealing the
// one-time inbound token and outbound secret.
func NewCreatedConnectionResponse(c *chentity.ChannelConnection) CreatedConnectionResponse {
	return CreatedConnectionResponse{
		ConnectionResponse: NewConnectionResponse(c),
		InboundToken:       c.InboundToken,
		OutboundSecret:     c.Secret,
	}
}

// RotatedInboundTokenResponse is returned by POST /v1/channels/{id}/rotate-inbound-token.
// It is the only place the freshly issued integration token is revealed.
type RotatedInboundTokenResponse struct {
	InboundToken string `json:"inbound_token"`
}

// NewRotatedInboundTokenResponse maps the one-time plaintext token.
func NewRotatedInboundTokenResponse(c *chentity.ChannelConnection) RotatedInboundTokenResponse {
	return RotatedInboundTokenResponse{InboundToken: c.InboundToken}
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
// The integration token is supplied via the X-Inbound-Token header (preferred)
// or the inbound_token body field; integration_key/webhook_verify_token are
// accepted as legacy aliases.
type InboundRequest struct {
	TenantKey          string           `json:"tenant_key"`
	InboundToken       string           `json:"inbound_token"`
	IntegrationKey     string           `json:"integration_key"`
	WebhookVerifyToken string           `json:"webhook_verify_token"`
	ExternalMessageID  string           `json:"external_message_id"`
	ExternalContactID  string           `json:"external_contact_id"`
	ContactName        string           `json:"contact_name"`
	ContactPhone       string           `json:"contact_phone"`
	ContactDocument    string           `json:"contact_document"`
	Channel            string           `json:"channel"`
	Text               string           `json:"text"`
	Attachments        []AttachmentItem `json:"attachments"`
	Metadata           map[string]any   `json:"metadata"`
	Timestamp          int64            `json:"timestamp"`
}

// Token returns the integration token from the body, preferring the canonical
// inbound_token field over the legacy aliases.
func (r InboundRequest) Token() string {
	switch {
	case r.InboundToken != "":
		return r.InboundToken
	case r.WebhookVerifyToken != "":
		return r.WebhookVerifyToken
	default:
		return r.IntegrationKey
	}
}

// ToMessage maps the request to the domain inbound message.
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
