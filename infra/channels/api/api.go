// Package api is the generic HTTP "API channel" adapter. Any external system
// integrates with it the same way: it POSTs inbound messages to the inbound
// endpoint (authenticated by the connection's inbound_token + an HMAC signature),
// and chat DELIVERS outbound messages by POSTing a signed envelope to the
// company's configured webhook (conn.BaseURL == outbound_url). The body is
// HMAC-SHA256 signed with the connection's outbound_secret (conn.Secret).
//
// It is the generic production default; WhatsApp/Telegram will be later, separate
// adapters of the same contracts.Adapter interface.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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

// attachment mirrors Chatwoot's webhook attachment: data_url + file_type let a
// Chatwoot-speaking receiver process media without adaptation; url/filename/size
// are kept for our own back-compat.
type attachment struct {
	URL         string `json:"url"`
	DataURL     string `json:"data_url,omitempty"`  // Chatwoot alias of the media URL
	FileType    string `json:"file_type,omitempty"` // image|audio|video|file
	ContentType string `json:"content_type,omitempty"`
	Filename    string `json:"filename,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

// fileTypeFor derives Chatwoot's attachment file_type from a MIME content type.
func fileTypeFor(contentType string) string {
	switch ct := strings.ToLower(contentType); {
	case strings.HasPrefix(ct, "image/"):
		return "image"
	case strings.HasPrefix(ct, "audio/"):
		return "audio"
	case strings.HasPrefix(ct, "video/"):
		return "video"
	default:
		return "file" // Chatwoot uses "file" for documents/other
	}
}

type contact struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	Phone      string `json:"phone,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
}

// messageBody mirrors Chatwoot's outgoing message: content + message_type +
// private + attachments[]; text is kept as a back-compat alias of content.
type messageBody struct {
	Content     string       `json:"content"`
	Text        string       `json:"text"`
	MessageType string       `json:"message_type"`        // "outgoing" (agent → customer)
	Private     bool         `json:"private"`             // internal notes are never delivered
	FileType    string       `json:"file_type,omitempty"` // message-level (first attachment)
	Attachments []attachment `json:"attachments,omitempty"`
	// Template carries the integrator template id + filled params for a WhatsApp
	// template send. Set only for templates; content/text/attachments are empty
	// then (the integrator already has the model and renders/sends to Meta).
	Template *templatePayload `json:"template,omitempty"`
}

// templatePayload is the outbound template section: id + params ONLY.
type templatePayload struct {
	ID     string            `json:"id"`
	Params map[string]string `json:"params,omitempty"`
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
	msgFileType := ""
	for _, at := range send.Attachments {
		ft := fileTypeFor(at.ContentType)
		if msgFileType == "" {
			msgFileType = ft
		}
		atts = append(atts, attachment{
			URL: at.URL, DataURL: at.URL, FileType: ft,
			ContentType: at.ContentType, Filename: at.Filename, Size: at.Size,
		})
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
		Message:   outboundMessageBody(send, atts, msgFileType),
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

// VerifyInbound validates the inbound POST. The connection is already resolved
// and authenticated by its inbound_token, so the HMAC body signature is OPTIONAL
// (Chatwoot-style multipart senders don't sign): when no signature/secret header
// is present, the token alone authenticates; when one IS present it must be valid.
// This is what lets multipart/form-data inbound work without a body HMAC.
func (a *Adapter) VerifyInbound(conn *chentity.ChannelConnection, rawBody []byte, headers map[string]string) error {
	if headers["X-Signature"] == "" && headers["X-Integration-Secret"] == "" {
		return nil
	}
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

// outboundMessageBody builds the message section. For a template send it carries
// ONLY the template id + params (no resolved text/structure); otherwise the text +
// attachments.
func outboundMessageBody(send chcontracts.OutboundSend, atts []attachment, fileType string) messageBody {
	if send.Template != nil {
		return messageBody{
			MessageType: "outgoing",
			Template:    &templatePayload{ID: send.Template.ID, Params: send.Template.Params},
		}
	}
	return messageBody{
		Content: send.Text, Text: send.Text, MessageType: "outgoing", Private: false,
		FileType: fileType, Attachments: atts,
	}
}
