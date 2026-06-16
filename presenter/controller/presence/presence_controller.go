// Package presence holds the HTTP controller for the agent presence endpoints.
package presence

import (
	"context"
	"net/http"
	"strings"

	presenceentity "github.com/romerito007/chat-smsnet-omnichannel/domain/presence/entity"
	presenceservice "github.com/romerito007/chat-smsnet-omnichannel/domain/presence/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	dto "github.com/romerito007/chat-smsnet-omnichannel/presenter/contracts/presence"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// AgentDirectory resolves user ids to display cards (name + signed avatar URL)
// so presence rows render the agent instead of a raw user id. Satisfied by the
// IAM user service (AgentCards). Optional: when unset, rows omit display info.
type AgentDirectory interface {
	AgentCards(ctx context.Context, userIDs []string) (map[string]shared.DisplayCard, error)
}

// Controller serves the agent presence endpoints.
type Controller struct {
	presence *presenceservice.Service
	agents   AgentDirectory
}

// NewController builds the controller.
func NewController(presence *presenceservice.Service) *Controller {
	return &Controller{presence: presence}
}

// SetAgentDirectory wires the directory used to resolve agent display info on the
// presence list. Optional: when unset, rows carry only the raw user id.
func (c *Controller) SetAgentDirectory(d AgentDirectory) *Controller {
	c.agents = d
	return c
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
		"data": dto.NewPresenceResponsesWithCards(items, c.agentCards(r.Context(), items)),
	})
}

// agentCards best-effort resolves the display cards for the listed agents. A
// directory error degrades gracefully to no cards (raw ids) rather than failing
// the whole list.
func (c *Controller) agentCards(ctx context.Context, items []*presenceentity.AgentPresence) map[string]shared.DisplayCard {
	if c.agents == nil || len(items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, p := range items {
		if p.UserID != "" {
			ids = append(ids, p.UserID)
		}
	}
	cards, err := c.agents.AgentCards(ctx, ids)
	if err != nil {
		return nil
	}
	return cards
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
