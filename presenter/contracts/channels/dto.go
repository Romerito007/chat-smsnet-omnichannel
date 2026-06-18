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
	Type              string                `json:"type"`
	Name              string                `json:"name"`
	BaseURL           string                `json:"base_url"`
	OutboundURL       string                `json:"outbound_url"`
	AuthType          string                `json:"auth_type"`
	Secret            string                `json:"secret"`
	OutboundSecret    string                `json:"outbound_secret"`
	BusinessHours     map[string]any        `json:"business_hours"`
	OutOfHoursMessage string                `json:"out_of_hours_message"`
	UsesProtocol      bool                  `json:"uses_protocol"`
	WhatsAppTemplates []WhatsAppTemplateDTO `json:"whatsapp_templates"`
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
		Type:              chentity.Type(r.Type),
		Name:              r.Name,
		BaseURL:           baseURL,
		AuthType:          chentity.AuthType(r.AuthType),
		Secret:            secret,
		BusinessHours:     r.BusinessHours,
		OutOfHoursMessage: r.OutOfHoursMessage,
		UsesProtocol:      r.UsesProtocol,
		WhatsAppTemplates: templatesToEntity(r.WhatsAppTemplates),
	}
}

// UpdateConnectionRequest is the body of PATCH /v1/channels/{id}.
type UpdateConnectionRequest struct {
	Name              *string                `json:"name"`
	Status            *string                `json:"status"`
	BaseURL           *string                `json:"base_url"`
	OutboundURL       *string                `json:"outbound_url"`
	AuthType          *string                `json:"auth_type"`
	Secret            *string                `json:"secret"`
	OutboundSecret    *string                `json:"outbound_secret"`
	BusinessHours     *map[string]any        `json:"business_hours"`
	OutOfHoursMessage *string                `json:"out_of_hours_message"`
	Enabled           *bool                  `json:"enabled"`
	UsesProtocol      *bool                  `json:"uses_protocol"`
	WhatsAppTemplates *[]WhatsAppTemplateDTO `json:"whatsapp_templates"`
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
		Name:              r.Name,
		BaseURL:           baseURL,
		Secret:            secret,
		BusinessHours:     r.BusinessHours,
		OutOfHoursMessage: r.OutOfHoursMessage,
		Enabled:           r.Enabled,
		UsesProtocol:      r.UsesProtocol,
	}
	if r.Status != nil {
		st := chentity.Status(*r.Status)
		cmd.Status = &st
	}
	if r.AuthType != nil {
		at := chentity.AuthType(*r.AuthType)
		cmd.AuthType = &at
	}
	if r.WhatsAppTemplates != nil {
		ts := templatesToEntity(*r.WhatsAppTemplates)
		cmd.WhatsAppTemplates = &ts
	}
	return cmd
}

// ConnectionResponse is the public representation of a connection. Neither the
// outbound secret nor the inbound token is ever returned here (only whether each
// is set); both are revealed only once, on creation, via CreatedConnectionResponse.
type ConnectionResponse struct {
	ID                string                `json:"id"`
	TenantID          string                `json:"tenant_id"`
	Type              string                `json:"type"`
	Name              string                `json:"name,omitempty"`
	Status            string                `json:"status"`
	BaseURL           string                `json:"base_url,omitempty"`
	AuthType          string                `json:"auth_type,omitempty"`
	HasSecret         bool                  `json:"has_secret"`
	HasInboundToken   bool                  `json:"has_inbound_token"`
	BusinessHours     map[string]any        `json:"business_hours,omitempty"`
	OutOfHoursMessage string                `json:"out_of_hours_message"`
	Enabled           bool                  `json:"enabled"`
	UsesProtocol      bool                  `json:"uses_protocol"`
	WhatsAppTemplates []WhatsAppTemplateDTO `json:"whatsapp_templates,omitempty"`
	CreatedAt         time.Time             `json:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at"`
}

// NewConnectionResponse maps a connection, masking both the secret and token.
func NewConnectionResponse(c *chentity.ChannelConnection) ConnectionResponse {
	return ConnectionResponse{
		ID:                c.ID,
		TenantID:          c.TenantID,
		Type:              string(c.Type),
		Name:              c.Name,
		Status:            string(c.Status),
		BaseURL:           c.BaseURL,
		AuthType:          string(c.AuthType),
		HasSecret:         c.Secret != "",
		HasInboundToken:   c.InboundTokenHash != "",
		BusinessHours:     c.BusinessHours,
		OutOfHoursMessage: c.OutOfHoursMessage,
		Enabled:           c.Enabled,
		UsesProtocol:      c.UsesProtocol,
		WhatsAppTemplates: templatesFromEntity(c.WhatsAppTemplates),
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
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

// RotatedOutboundSecretResponse is returned by POST /v1/channels/{id}/rotate-outbound-secret.
// It is the only place the freshly issued outbound HMAC secret is revealed; the old
// secret stops working, so the integrator must switch to this value.
type RotatedOutboundSecretResponse struct {
	OutboundSecret string `json:"outbound_secret"`
}

// NewRotatedOutboundSecretResponse maps the one-time plaintext outbound secret.
func NewRotatedOutboundSecretResponse(c *chentity.ChannelConnection) RotatedOutboundSecretResponse {
	return RotatedOutboundSecretResponse{OutboundSecret: c.Secret}
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
	TenantKey          string `json:"tenant_key"`
	InboundToken       string `json:"inbound_token"`
	IntegrationKey     string `json:"integration_key"`
	WebhookVerifyToken string `json:"webhook_verify_token"`
	ExternalMessageID  string `json:"external_message_id"`
	ExternalContactID  string `json:"external_contact_id"`
	ContactName        string `json:"contact_name"`
	ContactPhone       string `json:"contact_phone"`
	ContactDocument    string `json:"contact_document"`
	Channel            string `json:"channel"`
	// Group fields (all optional): GroupJID present ("...@g.us") marks a GROUP
	// message; Sender* identify the member who sent it. Absent = 1:1 (unchanged).
	GroupJID         string                   `json:"group_jid"`
	SenderJID        string                   `json:"sender_jid"`
	SenderName       string                   `json:"sender_name"`
	SenderPhone      string                   `json:"sender_phone"`
	Text             string                   `json:"text"`
	Attachments      []AttachmentItem         `json:"attachments"`
	Contacts         []conventity.ContactCard `json:"contacts"`
	Location         *conventity.Location     `json:"location"`
	InteractiveReply *InteractiveReplyItem    `json:"interactive_reply"`
	Metadata         map[string]any           `json:"metadata"`
	Timestamp        int64                    `json:"timestamp"`
}

// InteractiveReplyItem is the inbound interactive reply on the wire: the chosen
// button/list id+title (+description for list) and the menu's context id (Meta
// context.id, i.e. the external id of the menu message the chat sent).
type InteractiveReplyItem struct {
	Kind              string `json:"kind"` // "button" | "list"
	ID                string `json:"id"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	ContextExternalID string `json:"context_external_id"`
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
		GroupJID:          r.GroupJID,
		SenderJID:         r.SenderJID,
		SenderName:        r.SenderName,
		SenderPhone:       r.SenderPhone,
		Text:              r.Text,
		Attachments:       atts,
		Contacts:          r.Contacts,
		Location:          r.Location,
		InteractiveReply:  r.InteractiveReply.toContract(),
		Metadata:          r.Metadata,
		Timestamp:         r.Timestamp,
	}
}

func (r *InteractiveReplyItem) toContract() *chcontracts.InboundInteractiveReply {
	if r == nil {
		return nil
	}
	return &chcontracts.InboundInteractiveReply{
		Kind: r.Kind, ID: r.ID, Title: r.Title, Description: r.Description, ContextExternalID: r.ContextExternalID,
	}
}

// ── whatsapp templates (render-only mirror) ─────────────────────────────────

// WhatsAppTemplateDTO mirrors entity.WhatsAppTemplate on the wire.
type WhatsAppTemplateDTO struct {
	ID       string                   `json:"id"`
	Name     string                   `json:"name"`
	Language string                   `json:"language,omitempty"`
	Category string                   `json:"category,omitempty"`
	Body     WhatsAppTemplateBodyDTO  `json:"body"`
	Header   *WhatsAppTemplateHeader  `json:"header,omitempty"`
	Buttons  []WhatsAppTemplateButton `json:"buttons,omitempty"`
	Footer   string                   `json:"footer,omitempty"`
}

type WhatsAppTemplateBodyDTO struct {
	Text      string                     `json:"text"`
	Variables []WhatsAppTemplateVariable `json:"variables,omitempty"`
}

type WhatsAppTemplateVariable struct {
	Key     string `json:"key"`
	Label   string `json:"label,omitempty"`
	Example string `json:"example,omitempty"`
}

type WhatsAppTemplateHeader struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type WhatsAppTemplateButton struct {
	Type string `json:"type"`
	Text string `json:"text"`
	URL  string `json:"url,omitempty"`
}

// TemplatesToEntity maps a wire template list to entities. Exported for the inbound
// templates-push endpoint (the gateway PUTs the mirror).
func TemplatesToEntity(in []WhatsAppTemplateDTO) []chentity.WhatsAppTemplate {
	return templatesToEntity(in)
}

func templatesToEntity(in []WhatsAppTemplateDTO) []chentity.WhatsAppTemplate {
	if in == nil {
		return nil
	}
	out := make([]chentity.WhatsAppTemplate, len(in))
	for i, t := range in {
		vars := make([]chentity.WhatsAppTemplateVariable, len(t.Body.Variables))
		for j, v := range t.Body.Variables {
			vars[j] = chentity.WhatsAppTemplateVariable{Key: v.Key, Label: v.Label, Example: v.Example}
		}
		btns := make([]chentity.WhatsAppTemplateButton, len(t.Buttons))
		for j, b := range t.Buttons {
			btns[j] = chentity.WhatsAppTemplateButton{Type: b.Type, Text: b.Text, URL: b.URL}
		}
		e := chentity.WhatsAppTemplate{
			ID: t.ID, Name: t.Name, Language: t.Language, Category: t.Category,
			Body:    chentity.WhatsAppTemplateBody{Text: t.Body.Text, Variables: vars},
			Buttons: btns, Footer: t.Footer,
		}
		if t.Header != nil {
			e.Header = &chentity.WhatsAppTemplateHeader{Type: t.Header.Type, Text: t.Header.Text}
		}
		out[i] = e
	}
	return out
}

func templatesFromEntity(in []chentity.WhatsAppTemplate) []WhatsAppTemplateDTO {
	if len(in) == 0 {
		return nil
	}
	out := make([]WhatsAppTemplateDTO, len(in))
	for i, t := range in {
		vars := make([]WhatsAppTemplateVariable, len(t.Body.Variables))
		for j, v := range t.Body.Variables {
			vars[j] = WhatsAppTemplateVariable{Key: v.Key, Label: v.Label, Example: v.Example}
		}
		btns := make([]WhatsAppTemplateButton, len(t.Buttons))
		for j, b := range t.Buttons {
			btns[j] = WhatsAppTemplateButton{Type: b.Type, Text: b.Text, URL: b.URL}
		}
		d := WhatsAppTemplateDTO{
			ID: t.ID, Name: t.Name, Language: t.Language, Category: t.Category,
			Body:    WhatsAppTemplateBodyDTO{Text: t.Body.Text, Variables: vars},
			Buttons: btns, Footer: t.Footer,
		}
		if t.Header != nil {
			d.Header = &WhatsAppTemplateHeader{Type: t.Header.Type, Text: t.Header.Text}
		}
		out[i] = d
	}
	return out
}
