// Package mock is a functional channel adapter used for development, testing and
// as the fallback for channel types without a dedicated adapter. SendMessage
// succeeds with a generated external id; a message whose text starts with "FAIL"
// returns an error, to exercise the retry/failure path.
package mock

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Adapter is the functional mock channel adapter.
type Adapter struct {
	channelType chentity.Type
}

// New builds the mock adapter for a given type (used as a fallback for several).
func New(t chentity.Type) *Adapter {
	if t == "" {
		t = chentity.TypeCustom
	}
	return &Adapter{channelType: t}
}

// Type implements contracts.Adapter.
func (a *Adapter) Type() chentity.Type { return a.channelType }

// SendMessage simulates delivery. "FAIL" prefix forces an error.
func (a *Adapter) SendMessage(_ context.Context, _ *chentity.ChannelConnection, send chcontracts.OutboundSend) (chcontracts.SendResult, error) {
	if strings.HasPrefix(send.Text, "FAIL") {
		return chcontracts.SendResult{}, errors.New("mock channel send failure")
	}
	return chcontracts.SendResult{
		ExternalMessageID: "mock-" + shared.NewID(),
		Status:            chentity.DeliverySent,
	}, nil
}

// VerifyInbound validates the HMAC signature or exact secret when the connection
// has a secret; otherwise resolution by webhook token suffices.
func (a *Adapter) VerifyInbound(conn *chentity.ChannelConnection, rawBody []byte, headers map[string]string) error {
	return VerifySignature(conn.Secret, rawBody, headers)
}

// ParseDeliveryReceipt accepts {"receipts":[...]}, a bare array, or a single
// {external_message_id,status,error} object.
func (a *Adapter) ParseDeliveryReceipt(rawBody []byte) ([]chcontracts.DeliveryReceipt, error) {
	return ParseReceipts(rawBody)
}

// ── shared helpers (also used by the structural adapters) ─────────────────────

// VerifySignature validates X-Signature (HMAC) or X-Integration-Secret against
// the secret. An empty secret means token-resolution already authenticated.
func VerifySignature(secret string, rawBody []byte, headers map[string]string) error {
	if secret == "" {
		return nil
	}
	if sig := headers["X-Signature"]; sig != "" {
		sig = strings.TrimPrefix(sig, "sha256=")
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(rawBody)
		expected := hex.EncodeToString(mac.Sum(nil))
		if subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(sig))) == 1 {
			return nil
		}
		return errors.New("invalid signature")
	}
	if sec := headers["X-Integration-Secret"]; sec != "" {
		if subtle.ConstantTimeCompare([]byte(sec), []byte(secret)) == 1 {
			return nil
		}
		return errors.New("invalid secret")
	}
	return errors.New("missing signature")
}

type receiptItem struct {
	ExternalMessageID string `json:"external_message_id"`
	Status            string `json:"status"`
	Error             string `json:"error"`
}

type receiptEnvelope struct {
	Receipts          []receiptItem `json:"receipts"`
	ExternalMessageID string        `json:"external_message_id"`
	Status            string        `json:"status"`
	Error             string        `json:"error"`
}

// ParseReceipts parses a generic delivery-receipt payload.
func ParseReceipts(rawBody []byte) ([]chcontracts.DeliveryReceipt, error) {
	var env receiptEnvelope
	if err := json.Unmarshal(rawBody, &env); err == nil {
		items := env.Receipts
		if len(items) == 0 && env.ExternalMessageID != "" {
			items = []receiptItem{{ExternalMessageID: env.ExternalMessageID, Status: env.Status, Error: env.Error}}
		}
		if len(items) > 0 {
			return toReceipts(items), nil
		}
	}
	// Try a bare array.
	var arr []receiptItem
	if err := json.Unmarshal(rawBody, &arr); err == nil && len(arr) > 0 {
		return toReceipts(arr), nil
	}
	return nil, errors.New("no receipts in payload")
}

func toReceipts(items []receiptItem) []chcontracts.DeliveryReceipt {
	out := make([]chcontracts.DeliveryReceipt, 0, len(items))
	for _, it := range items {
		out = append(out, chcontracts.DeliveryReceipt{
			ExternalMessageID: it.ExternalMessageID,
			Status:            chentity.DeliveryStatus(it.Status),
			Error:             it.Error,
		})
	}
	return out
}

var _ chcontracts.Adapter = (*Adapter)(nil)
