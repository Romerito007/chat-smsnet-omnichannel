// Package contracts holds the channels service inputs/outputs and task payloads.
package contracts

import "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"

// InboundMessage is the normalized payload of an inbound channel message.
type InboundMessage struct {
	TenantKey         string
	IntegrationKey    string
	ExternalMessageID string
	ExternalContactID string
	ContactName       string
	ContactPhone      string
	ContactDocument   string
	Channel           string
	Text              string
	Attachments       []entity.Attachment
	Metadata          map[string]any
	Timestamp         int64 // epoch millis; 0 means "now"
}

// InboundResult is returned by the inbound handler.
type InboundResult struct {
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	ContactID      string `json:"contact_id"`
	Status         string `json:"status"`
	Idempotent     bool   `json:"idempotent"`
}
