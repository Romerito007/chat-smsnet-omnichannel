package channels

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	channelservice "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/channels"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

const maxInboundBody = 1 << 20 // 1 MiB

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

// HandleMessage processes POST /v1/inbound/channel/{channel}/messages.
func (c *InboundController) HandleMessage(w http.ResponseWriter, r *http.Request) {
	channel := chi.URLParam(r, "channel")
	body, err := io.ReadAll(io.LimitReader(r.Body, maxInboundBody))
	if err != nil {
		middleware.WriteError(w, r, apperror.Validation("unreadable request body"))
		return
	}
	var req dto.InboundRequest
	if err := json.Unmarshal(body, &req); err != nil {
		middleware.WriteError(w, r, apperror.Validation("invalid JSON body").Wrap(err))
		return
	}

	// The integration token authenticates the channel without the front's JWT.
	// Header X-Inbound-Token is preferred; the body inbound_token is a fallback.
	token := r.Header.Get("X-Inbound-Token")
	if token == "" {
		token = req.Token()
	}
	conn, err := c.connections.ResolveInbound(r.Context(), token, chentity.Type(channel), body, verifyHeaders(r))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}

	ctx := shared.WithTenant(r.Context(), conn.TenantID)
	result, err := c.inbound.Handle(ctx, conn, req.ToMessage(channel))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, result)
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
