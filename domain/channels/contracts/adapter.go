package contracts

import (
	"context"

	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
)

// OutboundSend is the channel-agnostic payload an adapter delivers.
type OutboundSend struct {
	ExternalContactID string
	Text              string
	Attachments       []conventity.Attachment
	Metadata          map[string]any
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
