// Package contracts holds the channels service inputs/outputs and task payloads.
package contracts

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

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
	Attachments       []entity.Attachment // already-hosted media (URL mode)
	RawAttachments    []RawFile           // raw bytes (multipart mode), persisted on Handle
	Metadata          map[string]any
	Timestamp         int64 // epoch millis; 0 means "now"
}

// RawFile is a raw inbound attachment (Chatwoot multipart/form-data): the bytes
// plus filename and content-type, persisted to storage by the inbound handler.
type RawFile struct {
	Filename    string
	ContentType string
	Data        []byte
}

// InboundAttachmentStore persists a raw inbound file to storage and returns the
// hosted attachment (with its access-gated download URL). Implemented by the
// attachments service; consumed by the inbound handler after the conversation is
// resolved (so the record can be access-checked on download). Primitive args keep
// the channels domain decoupled from the attachments domain.
type InboundAttachmentStore interface {
	StoreInbound(ctx context.Context, conversationID, filename, contentType string, data []byte) (entity.Attachment, error)
}

// IntegrationMediaURLBuilder resolves a signed, JWT-less public URL for an
// attachment, used on the INTEGRATION rail (outbound delivery to an external
// system, which cannot use the internal JWT-gated download URL). Implemented by
// the attachments service.
type IntegrationMediaURLBuilder interface {
	IntegrationMediaURL(ctx context.Context, attachmentID string) (string, error)
}

// InboundResult is returned by the inbound handler.
type InboundResult struct {
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	ContactID      string `json:"contact_id"`
	Status         string `json:"status"`
	Idempotent     bool   `json:"idempotent"`
}
