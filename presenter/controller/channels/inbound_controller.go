package channels

import (
	"context"
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
	groupcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/groups/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/channels"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

const (
	maxInboundBody      = 1 << 20  // 1 MiB (JSON body)
	maxInboundMultipart = 30 << 20 // 30 MiB (multipart with media)
)

// ReceiptApplier advances a message's delivery status from an optional receipt,
// correlated by the chat's own message id. Implemented by the conversations
// service.
type ReceiptApplier interface {
	ApplyDeliveryReceipt(ctx context.Context, messageID, status string) error
}

// ContactIdentityUpdater persists a verified channel identity (e.g. a WhatsApp JID)
// onto a contact. Implemented by the contacts service. Idempotent and additive.
type ContactIdentityUpdater interface {
	AddChannelIdentity(ctx context.Context, contactID, channel, externalID string) (applied bool, err error)
}

// GroupSink idempotently upserts a gateway group-sync batch. Implemented by the
// groups service. Optional: a nil sink rejects the groups endpoint.
type GroupSink interface {
	UpsertBatch(ctx context.Context, channelID string, groups []groupcontracts.UpsertGroup) (int, error)
}

// InboundController serves the public, signature-authenticated inbound endpoints
// (messages and delivery receipts).
type InboundController struct {
	connections *channelservice.ConnectionService
	inbound     *channelservice.InboundService
	receipts    ReceiptApplier
	identities  ContactIdentityUpdater
	groups      GroupSink
}

// NewInboundController builds the controller.
func NewInboundController(connections *channelservice.ConnectionService, inbound *channelservice.InboundService, receipts ReceiptApplier, identities ContactIdentityUpdater, groups GroupSink) *InboundController {
	return &InboundController{connections: connections, inbound: inbound, receipts: receipts, identities: identities, groups: groups}
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
		GroupJID:          get("group_jid"),
		SenderJID:         get("sender_jid"),
		SenderName:        get("sender_name"),
		SenderPhone:       get("sender_phone"),
		Text:              get("content", "text"),
	}
	if ts := get("timestamp"); ts != "" {
		req.Timestamp, _ = strconv.ParseInt(ts, 10, 64)
	}
	// Rich structured payloads may ride a multipart body as JSON-encoded fields, so a
	// shared contact/location is materialized even without a file attachment.
	if v := get("contacts"); v != "" {
		_ = json.Unmarshal([]byte(v), &req.Contacts)
	}
	if v := get("location"); v != "" {
		_ = json.Unmarshal([]byte(v), &req.Location)
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

// HandleContactIdentity processes POST /v1/inbound/channel/{channel}/contact-identity.
// A channel integration (the WhatsApp gateway) calls it to persist a VERIFIED
// identity (e.g. a resolved JID) onto an existing contact, so later webhooks carry
// source=identity without re-verifying the phone. Edge-authenticated by the channel
// inbound token (NOT the front's JWT); the tenant comes only from that token.
func (c *InboundController) HandleContactIdentity(w http.ResponseWriter, r *http.Request) {
	channel := chi.URLParam(r, "channel")
	body, err := io.ReadAll(io.LimitReader(r.Body, maxInboundBody))
	if err != nil {
		middleware.WriteError(w, r, apperror.Validation("unreadable request body"))
		return
	}
	var req struct {
		InboundToken string `json:"inbound_token"`
		ContactID    string `json:"contact_id"`
		Channel      string `json:"channel"`
		ExternalID   string `json:"external_id"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		middleware.WriteError(w, r, apperror.Validation("invalid JSON body").Wrap(err))
		return
	}

	token := r.Header.Get("X-Inbound-Token")
	if token == "" {
		token = req.InboundToken
	}
	conn, err := c.connections.ResolveInbound(r.Context(), token, chentity.Type(channel), body, verifyHeaders(r))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	// The identity channel defaults to the path channel type (the connection's type).
	identityChannel := strings.TrimSpace(req.Channel)
	if identityChannel == "" {
		identityChannel = channel
	}

	ctx := shared.WithTenant(r.Context(), conn.TenantID)
	applied, err := c.identities.AddChannelIdentity(ctx, req.ContactID, identityChannel, req.ExternalID)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "applied": applied})
}

// HandleDeliveryReceipts processes POST /v1/inbound/channel/{channel}/delivery-receipts.
// This is the OPTIONAL status channel: the integrator reports a message's delivery
// status (delivered/read/failed) keyed by the chat's own message_id. Delivery
// itself does not depend on it — a message with no receipt simply has no status.
func (c *InboundController) HandleDeliveryReceipts(w http.ResponseWriter, r *http.Request) {
	channel := chi.URLParam(r, "channel")
	body, err := io.ReadAll(io.LimitReader(r.Body, maxInboundBody))
	if err != nil {
		middleware.WriteError(w, r, apperror.Validation("unreadable request body"))
		return
	}

	var req struct {
		InboundToken string `json:"inbound_token"`
		MessageID    string `json:"message_id"`
		Status       string `json:"status"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		middleware.WriteError(w, r, apperror.Validation("invalid JSON body").Wrap(err))
		return
	}

	token := r.Header.Get("X-Inbound-Token")
	if token == "" {
		token = req.InboundToken
	}
	conn, err := c.connections.ResolveInbound(r.Context(), token, chentity.Type(channel), body, verifyHeaders(r))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(req.MessageID) == "" {
		middleware.WriteError(w, r, apperror.Validation("message_id is required"))
		return
	}

	ctx := shared.WithTenant(r.Context(), conn.TenantID)
	if err := c.receipts.ApplyDeliveryReceipt(ctx, req.MessageID, req.Status); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// groupBatchItem is one group in a gateway sync batch, in the gateway's shape. The
// gateway's field names (groupId, subject, participants, …) are mapped here onto our
// contract. Participants/admins are kept as raw strings (metadata, not contacts).
type groupBatchItem struct {
	GroupID      string   `json:"groupId"`
	GroupJID     string   `json:"group_jid"`
	Subject      string   `json:"subject"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Participants []string `json:"participants"`
	GroupAdmins  []string `json:"group_admins"`
	Admins       []string `json:"admins"`
	CompanyID    string   `json:"company_id"`
	WhatsAppWID  string   `json:"whatsapp_wid"`
	OwnerName    string   `json:"owner_name"`
	OwnerJID     string   `json:"owner_jid"`
	Activated    bool     `json:"activated"`
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (it groupBatchItem) toUpsert() groupcontracts.UpsertGroup {
	admins := it.GroupAdmins
	if len(admins) == 0 {
		admins = it.Admins
	}
	return groupcontracts.UpsertGroup{
		GroupJID:     firstNonEmpty(it.GroupJID, it.GroupID),
		Name:         firstNonEmpty(it.Name, it.Subject),
		Description:  it.Description,
		Participants: it.Participants,
		GroupAdmins:  admins,
		CompanyID:    it.CompanyID,
		WhatsAppWID:  it.WhatsAppWID,
		OwnerName:    it.OwnerName,
		OwnerJID:     it.OwnerJID,
		Activated:    it.Activated,
	}
}

// HandleGroups processes POST /v1/inbound/channel/{channel}/groups. The WhatsApp
// gateway calls it (in response to a group_sync_requested) to push the channel's
// group list, in batches. It is edge-authenticated by the channel inbound token (NOT
// the front's JWT); the tenant comes only from that token. The upsert is idempotent
// by (tenant, group_jid) and never resets the operator's attend choice.
func (c *InboundController) HandleGroups(w http.ResponseWriter, r *http.Request) {
	channel := chi.URLParam(r, "channel")
	body, err := io.ReadAll(io.LimitReader(r.Body, maxInboundBody))
	if err != nil {
		middleware.WriteError(w, r, apperror.Validation("unreadable request body"))
		return
	}
	var req struct {
		InboundToken string           `json:"inbound_token"`
		Groups       []groupBatchItem `json:"groups"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		middleware.WriteError(w, r, apperror.Validation("invalid JSON body").Wrap(err))
		return
	}

	token := r.Header.Get("X-Inbound-Token")
	if token == "" {
		token = req.InboundToken
	}
	conn, err := c.connections.ResolveInbound(r.Context(), token, chentity.Type(channel), body, verifyHeaders(r))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	if c.groups == nil {
		middleware.WriteError(w, r, apperror.Integration("groups sync is not configured"))
		return
	}

	upserts := make([]groupcontracts.UpsertGroup, 0, len(req.Groups))
	for _, it := range req.Groups {
		upserts = append(upserts, it.toUpsert())
	}
	ctx := shared.WithTenant(r.Context(), conn.TenantID)
	n, err := c.groups.UpsertBatch(ctx, conn.ID, upserts)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "upserted": n})
}

// HandleTemplates processes PUT /v1/inbound/channel/{channel}/templates. The WhatsApp
// gateway PUSHES the channel's current WhatsApp template mirror here (the chat no
// longer pulls). It is edge-authenticated by the channel inbound token (NOT the
// front's JWT); the tenant/channel come only from that token. The list REPLACES the
// channel's whatsapp_templates wholesale (it is a mirror) and alerts the agents.
func (c *InboundController) HandleTemplates(w http.ResponseWriter, r *http.Request) {
	channel := chi.URLParam(r, "channel")
	body, err := io.ReadAll(io.LimitReader(r.Body, maxInboundBody))
	if err != nil {
		middleware.WriteError(w, r, apperror.Validation("unreadable request body"))
		return
	}
	var req struct {
		InboundToken string                    `json:"inbound_token"`
		Templates    []dto.WhatsAppTemplateDTO `json:"templates"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		middleware.WriteError(w, r, apperror.Validation("invalid JSON body").Wrap(err))
		return
	}

	token := r.Header.Get("X-Inbound-Token")
	if token == "" {
		token = req.InboundToken
	}
	conn, err := c.connections.ResolveInbound(r.Context(), token, chentity.Type(channel), body, verifyHeaders(r))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}

	ctx := shared.WithTenant(r.Context(), conn.TenantID)
	updated, err := c.connections.ReplaceTemplates(ctx, conn.ID, dto.TemplatesToEntity(req.Templates))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "count": len(updated.WhatsAppTemplates)})
}
