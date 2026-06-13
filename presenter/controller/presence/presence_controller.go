// Package presence holds the HTTP controller for the agent presence endpoints.
package presence

import (
	"net/http"
	"strings"

	presenceentity "github.com/romerito007/chat-smsnet-omnichannel/domain/presence/entity"
	presenceservice "github.com/romerito007/chat-smsnet-omnichannel/domain/presence/service"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/presence"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the agent presence endpoints.
type Controller struct {
	presence *presenceservice.Service
}

// NewController builds the controller.
func NewController(presence *presenceservice.Service) *Controller {
	return &Controller{presence: presence}
}

// List handles GET /v1/agents/presence. Optional ?sector_id= scopes the result
// to the agents of that sector (server-side), instead of the whole team.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	items, err := c.presence.List(r.Context(), strings.TrimSpace(r.URL.Query().Get("sector_id")))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{
		"data": dto.NewPresenceResponses(items),
	})
}

// SetStatus handles POST /v1/agents/presence/status.
func (c *Controller) SetStatus(w http.ResponseWriter, r *http.Request) {
	var req dto.SetStatusRequest
	if err := middleware.DecodeJSON(r, &req); err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	p, err := c.presence.SetStatus(r.Context(), req.UserID, presenceentity.Status(req.Status))
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	middleware.WriteJSON(w, http.StatusOK, dto.NewPresenceResponse(p))
}
