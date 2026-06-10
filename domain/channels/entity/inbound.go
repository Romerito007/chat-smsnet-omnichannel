package entity

import "time"

// InboundRecord is the idempotency ledger entry for a processed inbound message,
// keyed by (tenant, channel, external_message_id). It maps an external message to
// the conversation/message it produced, so re-delivery is a no-op.
type InboundRecord struct {
	ID                string
	TenantID          string
	Channel           string
	ExternalMessageID string
	ConversationID    string
	MessageID         string
	CreatedAt         time.Time
}
