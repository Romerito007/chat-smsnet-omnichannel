package models

import "time"

// ChannelConnection is the BSON document for a channel connection. The secret is
// stored encrypted (encrypted_secret); the integration token is stored only as a
// SHA-256 hash (inbound_token_hash), never in plaintext.
type ChannelConnection struct {
	Base              `bson:",inline"`
	Type              string             `bson:"type"`
	Name              string             `bson:"name,omitempty"`
	Status            string             `bson:"status"`
	BaseURL           string             `bson:"base_url,omitempty"`
	AuthType          string             `bson:"auth_type,omitempty"`
	EncryptedSecret   string             `bson:"encrypted_secret,omitempty"`
	InboundTokenHash  string             `bson:"inbound_token_hash"`
	BusinessHours     map[string]any     `bson:"business_hours,omitempty"`
	OutOfHoursMessage string             `bson:"out_of_hours_message,omitempty"`
	Enabled           bool               `bson:"enabled"`
	UsesProtocol      bool               `bson:"uses_protocol,omitempty"`
	WhatsAppTemplates []WhatsAppTemplate `bson:"whatsapp_templates,omitempty"`
}

// WhatsAppTemplate is the BSON sub-document mirroring an integrator WhatsApp
// template (render-only).
type WhatsAppTemplate struct {
	ID       string                   `bson:"id"`
	Name     string                   `bson:"name"`
	Language string                   `bson:"language,omitempty"`
	Category string                   `bson:"category,omitempty"`
	Body     WhatsAppTemplateBody     `bson:"body"`
	Header   *WhatsAppTemplateHeader  `bson:"header,omitempty"`
	Buttons  []WhatsAppTemplateButton `bson:"buttons,omitempty"`
	Footer   string                   `bson:"footer,omitempty"`
}

type WhatsAppTemplateBody struct {
	Text      string                     `bson:"text"`
	Variables []WhatsAppTemplateVariable `bson:"variables,omitempty"`
}

type WhatsAppTemplateVariable struct {
	Key     string `bson:"key"`
	Label   string `bson:"label,omitempty"`
	Example string `bson:"example,omitempty"`
}

type WhatsAppTemplateHeader struct {
	Type string `bson:"type"`
	Text string `bson:"text,omitempty"`
}

type WhatsAppTemplateButton struct {
	Type string `bson:"type"`
	Text string `bson:"text"`
	URL  string `bson:"url,omitempty"`
}

// InboundRecord is the BSON document for the inbound idempotency ledger.
type InboundRecord struct {
	ID                string    `bson:"_id"`
	TenantID          string    `bson:"tenant_id"`
	Channel           string    `bson:"channel"`
	ExternalMessageID string    `bson:"external_message_id"`
	ConversationID    string    `bson:"conversation_id"`
	MessageID         string    `bson:"message_id"`
	CreatedAt         time.Time `bson:"created_at"`
}
