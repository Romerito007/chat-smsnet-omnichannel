package models

import "time"

// ChannelConnection is the BSON document for a channel connection. The secret is
// stored encrypted (encrypted_secret); the integration token is stored only as a
// SHA-256 hash (inbound_token_hash), never in plaintext.
type ChannelConnection struct {
	Base              `bson:",inline"`
	Type              string         `bson:"type"`
	Name              string         `bson:"name,omitempty"`
	Status            string         `bson:"status"`
	BaseURL           string         `bson:"base_url,omitempty"`
	AuthType          string         `bson:"auth_type,omitempty"`
	EncryptedSecret   string         `bson:"encrypted_secret,omitempty"`
	InboundTokenHash  string         `bson:"inbound_token_hash"`
	DefaultSectorID   string         `bson:"default_sector_id,omitempty"`
	BusinessHours     map[string]any `bson:"business_hours,omitempty"`
	Enabled           bool           `bson:"enabled"`
	AutomationEnabled bool           `bson:"automation_enabled"`
}

// OutboundDelivery is the BSON document for an outbound delivery record.
type OutboundDelivery struct {
	ID                  string     `bson:"_id"`
	TenantID            string     `bson:"tenant_id"`
	ChannelConnectionID string     `bson:"channel_connection_id"`
	ConversationID      string     `bson:"conversation_id"`
	MessageID           string     `bson:"message_id"`
	Status              string     `bson:"status"`
	Attempts            int        `bson:"attempts"`
	ExternalMessageID   string     `bson:"external_message_id,omitempty"`
	LastError           string     `bson:"last_error,omitempty"`
	NextRetryAt         *time.Time `bson:"next_retry_at,omitempty"`
	CreatedAt           time.Time  `bson:"created_at"`
	UpdatedAt           time.Time  `bson:"updated_at"`
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
