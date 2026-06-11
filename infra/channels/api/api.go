// Package api is the generic HTTP "API channel" adapter. Any external system
// integrates with it the same way: it POSTs inbound messages to the inbound
// endpoint (authenticated by the connection's inbound_token + an HMAC signature),
// and chat DELIVERS outbound messages by POSTing a signed envelope to the
// company's configured webhook (conn.BaseURL == outbound_url). The body is
// HMAC-SHA256 signed with the connection's outbound_secret (conn.Secret).
//
// This is the production replacement for the old mock adapter. WhatsApp/Telegram
// are later, separate adapters of the same contracts.Adapter interface.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/channels/sign"
)

// eventMessage is the X-Chat-Event value for an outbound message delivery.
const eventMessage = "message.created"

// defaultTimeout bounds the outbound POST to the company webhook.
const defaultTimeout = 10 * time.Second

// Adapter implements contracts.Adapter for the generic API channel.
type Adapter struct {
	client      *http.Client
	channelType chentity.Type
}

// New builds the API adapter. The type defaults to TypeAPI; a different type may
// be passed so the adapter can also serve as the registry fallback.
func New(t chentity.Type) *Adapter {
	if t == "" {
		t = chentity.TypeAPI
	}
	return &Adapter{client: &http.Client{Timeout: defaultTimeout}, channelType: t}
}

// Type implements contracts.Adapter.
func (a *Adapter) Type() chentity.Type { return a.channelType }

// ── outbound envelope ─────────────────────────────────────────────────────────

type attachment struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type,omitempty"`
	Filename    string `json:"filename,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

type contact struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	Phone      string `json:"phone,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
}

type messageBody struct {
	Text        string       `json:"text"`
	Attachments []attachment `json:"attachments,omitempty"`
}

type envelope struct {
	DeliveryID     string         `json:"delivery_id"`
	ConversationID string         `json:"conversation_id"`
	Contact        contact        `json:"contact"`
	Message        messageBody    `json:"message"`
	Timestamp      int64          `json:"timestamp"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// outboundResponse is the optional acknowledgement body. external_message_id, if
// present, is recorded so later delivery receipts can be correlated.
type outboundResponse struct {
	ExternalMessageID string `json:"external_message_id"`
	MessageID         string `json:"message_id"`
}

// SendMessage POSTs the signed outbound envelope to conn.BaseURL (outbound_url).
// A 2xx response marks the message sent; any other outcome returns an error so
// the OutboundService retries with backoff. The external id comes from the
// response when provided, falling back to the delivery id.
func (a *Adapter) SendMessage(ctx context.Context, conn *chentity.ChannelConnection, send chcontracts.OutboundSend) (chcontracts.SendResult, error) {
	if conn.BaseURL == "" {
		return chcontracts.SendResult{}, fmt.Errorf("api channel: outbound_url not configured")
	}

	atts := make([]attachment, 0, len(send.Attachments))
	for _, at := range send.Attachments {
		atts = append(atts, attachment{URL: at.URL, ContentType: at.ContentType, Filename: at.Filename, Size: at.Size})
	}
	env := envelope{
		DeliveryID:     send.DeliveryID,
		ConversationID: send.ConversationID,
		Contact: contact{
			ID:         send.Contact.ID,
			Name:       send.Contact.Name,
			Phone:      send.Contact.Phone,
			ExternalID: firstNonEmpty(send.Contact.ExternalID, send.ExternalContactID),
		},
		Message:   messageBody{Text: send.Text, Attachments: atts},
		Timestamp: time.Now().UTC().UnixMilli(),
		Metadata:  send.Metadata,
	}
	body, err := json.Marshal(env)
	if err != nil {
		return chcontracts.SendResult{}, fmt.Errorf("api channel: marshal envelope: %w", err)
	}

	timestamp := strconv.FormatInt(env.Timestamp, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, conn.BaseURL, bytes.NewReader(body))
	if err != nil {
		return chcontracts.SendResult{}, fmt.Errorf("api channel: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Chat-Event", eventMessage)
	req.Header.Set("Timestamp", timestamp)
	req.Header.Set("Delivery-Id", send.DeliveryID)
	if conn.Secret != "" {
		req.Header.Set("Signature", sign.Payload(conn.Secret, timestamp, body))
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return chcontracts.SendResult{}, fmt.Errorf("api channel: POST %s: %w", conn.BaseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return chcontracts.SendResult{}, fmt.Errorf("api channel: outbound_url returned %d: %s", resp.StatusCode, truncate(respBody, 256))
	}

	externalID := send.DeliveryID
	var ack outboundResponse
	if err := json.Unmarshal(respBody, &ack); err == nil {
		if ack.ExternalMessageID != "" {
			externalID = ack.ExternalMessageID
		} else if ack.MessageID != "" {
			externalID = ack.MessageID
		}
	}
	return chcontracts.SendResult{ExternalMessageID: externalID, Status: chentity.DeliverySent}, nil
}

// VerifyInbound validates the inbound POST: the connection is already resolved by
// its inbound_token, and the body signature (HMAC of the body, optionally bound
// to a Timestamp header) is verified against the connection secret.
func (a *Adapter) VerifyInbound(conn *chentity.ChannelConnection, rawBody []byte, headers map[string]string) error {
	return sign.VerifySignature(conn.Secret, rawBody, headers)
}

// ParseDeliveryReceipt reads delivered/read/failed statuses keyed by
// external_message_id from the receipt payload.
func (a *Adapter) ParseDeliveryReceipt(rawBody []byte) ([]chcontracts.DeliveryReceipt, error) {
	return sign.ParseReceipts(rawBody)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}

var _ chcontracts.Adapter = (*Adapter)(nil)
