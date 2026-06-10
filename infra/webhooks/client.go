package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
	infrahttp "github.com/romerito007/chat-smsnet-omnichannel/infra/http_client"
)

// Header names for signed webhook deliveries.
const (
	HeaderEvent      = "X-Webhook-Event"
	HeaderTimestamp  = "X-Webhook-Timestamp"
	HeaderSignature  = "X-Webhook-Signature"
	HeaderDeliveryID = "X-Webhook-Delivery-Id"
)

// Sender is the HTTP implementation of contracts.Sender. It POSTs the delivery's
// raw JSON body to the subscription URL and signs it with HMAC-SHA256.
type Sender struct {
	client *http.Client
}

// NewSender builds the sender.
func NewSender() *Sender {
	return &Sender{client: infrahttp.New(10 * time.Second)}
}

// Send POSTs the signed payload and returns the HTTP status code. The signature
// is HMAC-SHA256(secret, "<timestamp>.<body>"), hex-encoded, so receivers can
// verify integrity and bound replay using the timestamp header.
func (s *Sender) Send(ctx context.Context, sub *entity.WebhookSubscription, delivery *entity.WebhookDelivery) (contracts.SendResult, error) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	signature := Sign(sub.Secret, ts, delivery.Payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(delivery.Payload))
	if err != nil {
		return contracts.SendResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "chat-smsnet-webhooks/1")
	req.Header.Set(HeaderEvent, delivery.Event)
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderSignature, "sha256="+signature)
	req.Header.Set(HeaderDeliveryID, delivery.ID)

	resp, err := s.client.Do(req)
	if err != nil {
		return contracts.SendResult{}, err
	}
	defer resp.Body.Close()
	return contracts.SendResult{StatusCode: resp.StatusCode}, nil
}

// Sign computes the hex HMAC-SHA256 of "<timestamp>.<body>" with the secret.
func Sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

var _ contracts.Sender = (*Sender)(nil)
