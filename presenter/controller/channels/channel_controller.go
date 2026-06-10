// Package channels holds the HTTP controllers for channel integration
// management and the inbound endpoint.
package channels

import (
	"net/http"

	channelservice "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/channels"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves channel integration management (authenticated, channel.manage).
type Controller struct {
	channels *channelservice.ChannelService
}

// NewController builds the controller.
func NewController(channels *channelservice.ChannelService) *Controller {
	return &Controller{channels: channels}
}

// Create handles POST /v1/channels. The response includes the secret (once).
func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateIntegrationRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	integration, err := c.channels.Create(r.Context(), req.ToCommand())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusCreated, dto.NewIntegrationResponse(integration, true))
}

// List handles GET /v1/channels (secrets omitted).
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	page := middleware.PageFromRequest(r)
	items, err := c.channels.List(r.Context(), page)
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	resp := shared.NewPage(dto.NewIntegrationResponses(items), page.Limit, func(it dto.IntegrationResponse) shared.Cursor {
		return shared.Cursor{CreatedAt: it.CreatedAt.UnixMilli(), ID: it.ID}
	})
	middleware.WriteJSON(w, http.StatusOK, resp)
}
