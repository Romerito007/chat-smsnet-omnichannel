// Package contracts holds the presence realtime event payloads.
package contracts

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/presence/entity"
)

// EventPresenceChanged is the realtime event name emitted when an agent's
// presence changes.
const EventPresenceChanged = "agent.presence_changed"

// PresenceChanged is the payload of the agent.presence_changed event.
type PresenceChanged struct {
	TenantID           string    `json:"tenant_id"`
	UserID             string    `json:"user_id"`
	Status             string    `json:"status"`
	CurrentLoad        int       `json:"current_load"`
	MaxConcurrentChats int       `json:"max_concurrent_chats"`
	LastSeenAt         time.Time `json:"last_seen_at"`
}

// NewPresenceChanged builds the event payload from a presence record.
func NewPresenceChanged(p *entity.AgentPresence) PresenceChanged {
	return PresenceChanged{
		TenantID:           p.TenantID,
		UserID:             p.UserID,
		Status:             string(p.Status),
		CurrentLoad:        p.CurrentLoad,
		MaxConcurrentChats: p.MaxConcurrentChats,
		LastSeenAt:         p.LastSeenAt,
	}
}
