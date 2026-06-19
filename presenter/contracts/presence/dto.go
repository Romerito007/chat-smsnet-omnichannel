// Package presence holds the request/response DTOs for the presence endpoints.
package presence

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/presence/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// SetStatusRequest is the body of POST /v1/agents/presence/status. It sets the
// agent's DURABLE manual availability (online|away|offline). UserID is optional:
// empty means the caller's own presence; a different value requires user.manage.
type SetStatusRequest struct {
	UserID string `json:"user_id"`
	Status string `json:"status"`
}

// SetAutoOfflineRequest is the body of PATCH /v1/agents/presence/auto-offline. It
// toggles whether the agent goes effective-offline when their last socket drops
// while availability is online. UserID is optional (self unless user.manage).
type SetAutoOfflineRequest struct {
	UserID      string `json:"user_id"`
	AutoOffline bool   `json:"auto_offline"`
}

// PresenceResponse is the public representation of an agent's presence. Name and
// AvatarURL carry the resolved agent display info so the dashboard renders the
// agent without a per-row lookup and never shows a raw user id; both are empty
// when the directory could not resolve the user.
type PresenceResponse struct {
	TenantID  string `json:"tenant_id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	// Status is the EFFECTIVE status (precedence applied); availability is the raw
	// manual choice and auto_offline the per-agent toggle, so the front renders both.
	Status             string    `json:"status"`
	Availability       string    `json:"availability"`
	AutoOffline        bool      `json:"auto_offline"`
	CurrentLoad        int       `json:"current_load"`
	MaxConcurrentChats int       `json:"max_concurrent_chats"`
	LastSeenAt         time.Time `json:"last_seen_at"`
}

// NewPresenceResponse maps a presence entity to its DTO.
func NewPresenceResponse(p *entity.AgentPresence) PresenceResponse {
	return PresenceResponse{
		TenantID:           p.TenantID,
		UserID:             p.UserID,
		Status:             string(p.Status),
		Availability:       p.Availability,
		AutoOffline:        p.AutoOffline,
		CurrentLoad:        p.CurrentLoad,
		MaxConcurrentChats: p.MaxConcurrentChats,
		LastSeenAt:         p.LastSeenAt,
	}
}

// NewPresenceResponses maps a slice of presence records.
func NewPresenceResponses(items []*entity.AgentPresence) []PresenceResponse {
	out := make([]PresenceResponse, len(items))
	for i, p := range items {
		out[i] = NewPresenceResponse(p)
	}
	return out
}

// NewPresenceResponsesWithCards maps a slice of presence records, enriching each
// row with the resolved agent display card (name + signed avatar URL) keyed by
// user id. Rows whose user id is absent from cards keep empty display fields.
func NewPresenceResponsesWithCards(items []*entity.AgentPresence, cards map[string]shared.DisplayCard) []PresenceResponse {
	out := make([]PresenceResponse, len(items))
	for i, p := range items {
		r := NewPresenceResponse(p)
		if card, ok := cards[p.UserID]; ok {
			r.Name = card.Name
			r.AvatarURL = card.AvatarURL
		}
		out[i] = r
	}
	return out
}
