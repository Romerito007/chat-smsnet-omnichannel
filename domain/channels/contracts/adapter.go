package contracts

import (
	"context"

	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// OutboundSend is the channel-agnostic payload an adapter delivers. DeliveryID,
// ConversationID and Contact let adapters that POST a full envelope (e.g. the API
// channel) build it without reaching back into other domains.
type OutboundSend struct {
	DeliveryID        string
	ConversationID    string
	ExternalContactID string
	Contact           OutboundContact
	Text              string
	Attachments       []conventity.Attachment
	Metadata          map[string]any
}

// OutboundContact is the minimal contact reference included in an outbound
// envelope. ExternalID is the contact's identifier on this channel.
type OutboundContact struct {
	ID         string
	Name       string
	Phone      string
	ExternalID string
}

// SendResult is the outcome of a successful send.
type SendResult struct {
	ExternalMessageID string
	Status            chentity.DeliveryStatus // typically "sent"
}

// DeliveryReceipt is a parsed delivery/read/failure notification from a channel.
type DeliveryReceipt struct {
	ExternalMessageID string
	Status            chentity.DeliveryStatus // delivered | read | failed
	Error             string
}

// Adapter is the per-channel integration port. Implementations live in
// infra/channels/<type>.
type Adapter interface {
	// Type is the channel type this adapter serves.
	Type() chentity.Type
	// SendMessage delivers an outbound message, returning the external id.
	SendMessage(ctx context.Context, conn *chentity.ChannelConnection, send OutboundSend) (SendResult, error)
	// VerifyInbound validates an inbound request/receipt's signature or token.
	VerifyInbound(conn *chentity.ChannelConnection, rawBody []byte, headers map[string]string) error
	// ParseDeliveryReceipt parses a delivery-receipt payload into receipts.
	ParseDeliveryReceipt(rawBody []byte) ([]DeliveryReceipt, error)
}

// AdapterRegistry resolves an adapter for a channel type.
type AdapterRegistry interface {
	For(t chentity.Type) Adapter
}
