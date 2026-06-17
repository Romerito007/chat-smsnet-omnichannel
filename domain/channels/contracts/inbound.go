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
	// GroupJID, when set (a WhatsApp group JID, "...@g.us"), marks this as a GROUP
	// message: the conversation is keyed by the group (not the sender), and Sender*
	// below records which member authored it. Empty = a normal 1:1 message.
	GroupJID       string
	SenderJID      string
	SenderName     string
	SenderPhone    string
	Text           string
	Attachments    []entity.Attachment // already-hosted media (URL mode)
	RawAttachments []RawFile           // raw bytes (multipart mode), persisted on Handle
	// Contacts (customer shared vCard[s]) and Location (customer shared a location)
	// are the typed structured inbound payloads, when the gateway forwards them.
	Contacts []entity.ContactCard
	Location *entity.Location
	// InteractiveReply is the customer's choice on an interactive menu we sent
	// (message_type=interactive_reply).
	InteractiveReply *InboundInteractiveReply
	Metadata         map[string]any
	Timestamp        int64 // epoch millis; 0 means "now"
}

// InboundInteractiveReply is the gateway's normalized form of a WhatsApp
// interactive button_reply/list_reply: the chosen id+title (+description for list)
// and ContextExternalID (Meta context.id — the external id of the menu message we
// sent), which the chat resolves to the internal menu message id.
type InboundInteractiveReply struct {
	Kind              string // "button" | "list"
	ID                string
	Title             string
	Description       string
	ContextExternalID string
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
	// Discarded is true when the message was intentionally dropped (e.g. a group that
	// is not attended or not synced): the endpoint still returns 200 so the gateway
	// does not retry, but nothing was persisted.
	Discarded bool `json:"discarded,omitempty"`
}
