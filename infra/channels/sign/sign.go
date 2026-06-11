// Package sign holds the HMAC-SHA256 signing and verification helpers shared by
// the structural channel adapters (api, webchat, whatsapp). Keeping them in a
// leaf package lets every adapter sign outbound deliveries and verify inbound
// requests without depending on any one adapter implementation.
package sign

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
)

// Body computes the HMAC-SHA256 of rawBody alone, hex-encoded.
func Body(secret string, rawBody []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	return hex.EncodeToString(mac.Sum(nil))
}

// Payload computes the HMAC-SHA256 of "timestamp.rawBody", hex-encoded. This is
// the signature carried in the outbound X-Signature/Signature header alongside
// the Timestamp header, binding the body to a moment in time.
func Payload(secret, timestamp string, rawBody []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(rawBody)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature validates an inbound request's authenticity against the
// connection secret. An empty secret means token-resolution already
// authenticated the caller. It accepts, in order:
//   - X-Signature as HMAC of "Timestamp.body" (when a Timestamp header is sent),
//   - X-Signature as HMAC of the raw body,
//   - X-Integration-Secret as the exact shared secret.
func VerifySignature(secret string, rawBody []byte, headers map[string]string) error {
	if secret == "" {
		return nil
	}
	if sig := headers["X-Signature"]; sig != "" {
		sig = strings.ToLower(strings.TrimPrefix(sig, "sha256="))
		if ts := headers["Timestamp"]; ts != "" {
			if subtle.ConstantTimeCompare([]byte(Payload(secret, ts, rawBody)), []byte(sig)) == 1 {
				return nil
			}
		}
		if subtle.ConstantTimeCompare([]byte(Body(secret, rawBody)), []byte(sig)) == 1 {
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

// ParseReceipts parses a generic delivery-receipt payload: {"receipts":[...]},
// a bare array, or a single {external_message_id,status,error} object.
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
