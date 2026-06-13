// Package agents exposes a lightweight, assignment-oriented directory of the
// tenant's agents (id, name, presence). It is intentionally separate from the
// admin /v1/users surface: it returns only what an assignment selector needs and
// is readable by anyone who can assign conversations (conversation.assign).
package agents

import (
	"net/http"
	"strings"

	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	presenceentity "github.com/romerito007/chat-smsnet-omnichannel/domain/presence/entity"
	presenceservice "github.com/romerito007/chat-smsnet-omnichannel/domain/presence/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/presenter/middleware"
)

// Controller serves the assignable-agents directory.
type Controller struct {
	users    *iamservice.UserService
	presence *presenceservice.Service
}

// NewController builds the controller.
func NewController(users *iamservice.UserService, presence *presenceservice.Service) *Controller {
	return &Controller{users: users, presence: presence}
}

// assignableAgent is the light item the assignment selector consumes.
type assignableAgent struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Status             string `json:"status"` // presence status, or "offline"
	CurrentLoad        int    `json:"current_load"`
	MaxConcurrentChats int    `json:"max_concurrent_chats"`
}

// List handles GET /v1/agents: the active tenant users merged with presence.
// With ?sector_id=<id> it returns only the agents assignable to that sector — the
// exact set the routing assign accepts (membership lives only in the backend), so
// the assignment selector receives no agent it would have to discard.
func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	sectorID := strings.TrimSpace(r.URL.Query().Get("sector_id"))
	var (
		users []*iamentity.User
		err   error
	)
	if sectorID != "" {
		users, err = c.users.ListBySector(r.Context(), sectorID)
	} else {
		users, err = c.users.List(r.Context(), shared.PageRequest{Limit: shared.MaxPageSize})
	}
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	pres, err := c.presence.List(r.Context())
	if err != nil {
		middleware.WriteError(w, r, err)
		return
	}
	byUser := make(map[string]*presenceentity.AgentPresence, len(pres))
	for _, p := range pres {
		byUser[p.UserID] = p
	}

	out := make([]assignableAgent, 0, len(users))
	for _, u := range users {
		if u.Status != iamentity.StatusActive {
			continue
		}
		a := assignableAgent{ID: u.ID, Name: u.Name, Status: "offline", MaxConcurrentChats: u.MaxConcurrentChats}
		if p, ok := byUser[u.ID]; ok {
			a.Status = string(p.Status)
			a.CurrentLoad = p.CurrentLoad
			if p.MaxConcurrentChats > 0 {
				a.MaxConcurrentChats = p.MaxConcurrentChats
			}
		}
		out = append(out, a)
	}
	middleware.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}
