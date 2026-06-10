package channels

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	channelservice "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/channels"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// maxInboundBody bounds the inbound payload size.
const maxInboundBody = 1 << 20 // 1 MiB

// InboundController serves the public, signature-authenticated inbound endpoint.
type InboundController struct {
	channels *channelservice.ChannelService
	inbound  *channelservice.InboundService
}

// NewInboundController builds the controller.
func NewInboundController(channels *channelservice.ChannelService, inbound *channelservice.InboundService) *InboundController {
	return &InboundController{channels: channels, inbound: inbound}
}

// Handle processes POST /v1/inbound/channel/{channel}/messages.
//
// The request is authenticated by the integration signature (HMAC over the raw
// body) or the exact secret header; the tenant is derived from the matched
// integration, never from the payload. Processing is fast and idempotent.
func (c *InboundController) Handle(w http.ResponseWriter, r *http.Request) {
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

	integration, err := c.channels.Authenticate(
		r.Context(),
		req.IntegrationKey,
		channel,
		string(body),
		r.Header.Get("X-Signature"),
		r.Header.Get("X-Integration-Secret"),
	)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}

	// Tenant comes from the verified integration.
	ctx := shared.WithTenant(r.Context(), integration.TenantID)
	result, err := c.inbound.Handle(ctx, integration, req.ToMessage(channel))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, result)
}
