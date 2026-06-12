package channels

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	channelservice "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/channels"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

const (
	maxInboundBody      = 1 << 20  // 1 MiB (JSON body)
	maxInboundMultipart = 30 << 20 // 30 MiB (multipart with media)
)

// InboundController serves the public, signature-authenticated inbound endpoints
// (messages and delivery receipts).
type InboundController struct {
	connections *channelservice.ConnectionService
	inbound     *channelservice.InboundService
	outbound    *channelservice.OutboundService
}

// NewInboundController builds the controller.
func NewInboundController(connections *channelservice.ConnectionService, inbound *channelservice.InboundService, outbound *channelservice.OutboundService) *InboundController {
	return &InboundController{connections: connections, inbound: inbound, outbound: outbound}
}

// verifyHeaders extracts the signature/secret headers an adapter checks.
func verifyHeaders(r *http.Request) map[string]string {
	return map[string]string{
		"X-Signature":          r.Header.Get("X-Signature"),
		"X-Integration-Secret": r.Header.Get("X-Integration-Secret"),
	}
}

// HandleMessage processes POST /v1/inbound/channel/{channel}/messages. It accepts
// two Chatwoot-compatible shapes: JSON (attachments by URL) and multipart/form-data
// (raw file attachments). The multipart shape mirrors Chatwoot's create-message
// API: content, message_type, file_type, attachments[] files plus our routing
// fields (external_message_id, external_contact_id/contact_phone, inbound_token).
func (c *InboundController) HandleMessage(w http.ResponseWriter, r *http.Request) {
	channel := chi.URLParam(r, "channel")

	var (
		req         dto.InboundRequest
		raw         []chcontracts.RawFile
		bodyForAuth []byte // signed JSON body for optional HMAC; nil for multipart
	)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		var err error
		if req, raw, err = parseMultipartInbound(r); err != nil {
			middleware.WriteError(w, r, err)
			return
		}
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxInboundBody))
		if err != nil {
			middleware.WriteError(w, r, apperror.Validation("unreadable request body"))
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			middleware.WriteError(w, r, apperror.Validation("invalid JSON body").Wrap(err))
			return
		}
		bodyForAuth = body
	}

	// The integration token authenticates the channel without the front's JWT.
	// Header X-Inbound-Token is preferred; the body inbound_token is a fallback.
	token := r.Header.Get("X-Inbound-Token")
	if token == "" {
		token = req.Token()
	}
	conn, err := c.connections.ResolveInbound(r.Context(), token, chentity.Type(channel), bodyForAuth, verifyHeaders(r))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}

	ctx := shared.WithTenant(r.Context(), conn.TenantID)
	msg := req.ToMessage(channel)
	msg.RawAttachments = raw
	result, err := c.inbound.Handle(ctx, conn, msg)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, result)
}

// parseMultipartInbound reads the Chatwoot-style multipart/form-data body into an
// InboundRequest (text/routing fields) and the raw attachment files.
func parseMultipartInbound(r *http.Request) (dto.InboundRequest, []chcontracts.RawFile, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxInboundMultipart)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		return dto.InboundRequest{}, nil, apperror.Validation("invalid multipart body").Wrap(err)
	}
	form := r.MultipartForm

	get := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(r.FormValue(k)); v != "" {
				return v
			}
		}
		return ""
	}
	req := dto.InboundRequest{
		InboundToken:      get("inbound_token"),
		ExternalMessageID: get("external_message_id"),
		ExternalContactID: get("external_contact_id"),
		ContactName:       get("contact_name"),
		ContactPhone:      get("contact_phone"),
		ContactDocument:   get("contact_document"),
		Text:              get("content", "text"),
	}
	if ts := get("timestamp"); ts != "" {
		req.Timestamp, _ = strconv.ParseInt(ts, 10, 64)
	}

	var raw []chcontracts.RawFile
	// Accept both "attachments[]" (Chatwoot) and "attachments" / "file" field names.
	headers := append(append(form.File["attachments[]"], form.File["attachments"]...), form.File["file"]...)
	for _, fh := range headers {
		f, err := fh.Open()
		if err != nil {
			return dto.InboundRequest{}, nil, apperror.Validation("could not read attachment").Wrap(err)
		}
		data, rerr := io.ReadAll(io.LimitReader(f, maxInboundMultipart))
		_ = f.Close()
		if rerr != nil {
			return dto.InboundRequest{}, nil, apperror.Validation("could not read attachment").Wrap(rerr)
		}
		ct := fh.Header.Get("Content-Type")
		if ct == "" {
			ct = "application/octet-stream"
		}
		raw = append(raw, chcontracts.RawFile{Filename: fh.Filename, ContentType: ct, Data: data})
	}
	return req, raw, nil
}

// HandleDeliveryReceipts processes POST /v1/inbound/channel/{channel}/delivery-receipts.
func (c *InboundController) HandleDeliveryReceipts(w http.ResponseWriter, r *http.Request) {
	channel := chi.URLParam(r, "channel")
	body, err := io.ReadAll(io.LimitReader(r.Body, maxInboundBody))
	if err != nil {
		middleware.WriteError(w, r, apperror.Validation("unreadable request body"))
		return
	}

	token := r.Header.Get("X-Inbound-Token")
	if token == "" {
		// Fall back to a body inbound_token (the receipts payload is otherwise opaque).
		var tok struct {
			InboundToken string `json:"inbound_token"`
		}
		_ = json.Unmarshal(body, &tok)
		token = tok.InboundToken
	}
	conn, err := c.connections.ResolveInbound(r.Context(), token, chentity.Type(channel), body, verifyHeaders(r))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}

	ctx := shared.WithTenant(r.Context(), conn.TenantID)
	applied, err := c.outbound.ProcessReceipts(ctx, conn, body)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"applied": applied})
}
