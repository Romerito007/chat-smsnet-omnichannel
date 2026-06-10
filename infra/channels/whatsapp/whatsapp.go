// Package whatsapp is the structural WhatsApp channel adapter. Inbound
// verification and receipt parsing are wired; SendMessage is a stub that returns
// an error until the provider HTTP client is implemented (the seam is here).
package whatsapp

import (
	"context"
	"errors"

	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/mock"
)

// Adapter is the WhatsApp adapter (structure only for outbound).
type Adapter struct{}

// New builds the adapter.
func New() *Adapter { return &Adapter{} }

// Type implements contracts.Adapter.
func (a *Adapter) Type() chentity.Type { return chentity.TypeWhatsApp }

// SendMessage is not yet implemented; this is the integration seam where the
// WhatsApp Cloud/BSP HTTP call will live (using conn.BaseURL + conn.Secret).
func (a *Adapter) SendMessage(_ context.Context, _ *chentity.ChannelConnection, _ chcontracts.OutboundSend) (chcontracts.SendResult, error) {
	return chcontracts.SendResult{}, errors.New("whatsapp adapter not configured")
}

// VerifyInbound validates the request signature/secret.
func (a *Adapter) VerifyInbound(conn *chentity.ChannelConnection, rawBody []byte, headers map[string]string) error {
	return mock.VerifySignature(conn.Secret, rawBody, headers)
}

// ParseDeliveryReceipt parses a delivery-receipt payload.
func (a *Adapter) ParseDeliveryReceipt(rawBody []byte) ([]chcontracts.DeliveryReceipt, error) {
	return mock.ParseReceipts(rawBody)
}

var _ chcontracts.Adapter = (*Adapter)(nil)
