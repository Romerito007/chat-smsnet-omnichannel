package models

import "time"

// Integration is the BSON document for a channel integration credential.
type Integration struct {
	Base              `bson:",inline"`
	Channel           string `bson:"channel"`
	Name              string `bson:"name,omitempty"`
	IntegrationKey    string `bson:"integration_key"`
	Secret            string `bson:"secret"`
	Enabled           bool   `bson:"enabled"`
	AutomationEnabled bool   `bson:"automation_enabled"`
	DefaultQueueID    string `bson:"default_queue_id,omitempty"`
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
