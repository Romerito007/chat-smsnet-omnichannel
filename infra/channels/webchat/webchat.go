// Package webchat is the webchat channel adapter. Outbound delivery to a web
// widget is acknowledged locally (the message reaches the widget over the
// WebSocket layer); inbound verification and receipt parsing are wired.
package webchat

import (
	"context"

	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/sign"
)

// Adapter is the webchat adapter.
type Adapter struct{}

// New builds the adapter.
func New() *Adapter { return &Adapter{} }

// Type implements contracts.Adapter.
func (a *Adapter) Type() chentity.Type { return chentity.TypeWebchat }

// SendMessage acknowledges delivery to the web widget with a generated id.
func (a *Adapter) SendMessage(_ context.Context, _ *chentity.ChannelConnection, _ chcontracts.OutboundSend) (chcontracts.SendResult, error) {
	return chcontracts.SendResult{
		ExternalMessageID: "webchat-" + shared.NewID(),
		Status:            chentity.DeliverySent,
	}, nil
}

// VerifyInbound validates the request signature/secret.
func (a *Adapter) VerifyInbound(conn *chentity.ChannelConnection, rawBody []byte, headers map[string]string) error {
	return sign.VerifySignature(conn.Secret, rawBody, headers)
}

// ParseDeliveryReceipt parses a delivery-receipt payload.
func (a *Adapter) ParseDeliveryReceipt(rawBody []byte) ([]chcontracts.DeliveryReceipt, error) {
	return sign.ParseReceipts(rawBody)
}

var _ chcontracts.Adapter = (*Adapter)(nil)
