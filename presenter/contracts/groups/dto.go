// Package groups holds the request/response DTOs for the WhatsApp groups
// management endpoints.
package groups

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/groups/entity"
)

// UpdateAttendRequest is the body of PATCH /v1/groups/{id}: mark a group to attend
// or not. Attend is required (the only mutable field in Phase 1).
type UpdateAttendRequest struct {
	Attend *bool `json:"attend"`
}

// SyncRequest is the body of POST /v1/groups/sync: ask the channel's gateway to
// push its group list. channel_id identifies the channel whose managed webhook
// receives the request.
type SyncRequest struct {
	ChannelID string `json:"channel_id"`
}

// GroupResponse is the public representation of a known WhatsApp group.
type GroupResponse struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	ChannelID    string    `json:"channel_id,omitempty"`
	GroupJID     string    `json:"group_jid"`
	Name         string    `json:"name,omitempty"`
	Description  string    `json:"description,omitempty"`
	Participants []string  `json:"participants"`
	GroupAdmins  []string  `json:"group_admins"`
	CompanyID    string    `json:"company_id,omitempty"`
	WhatsAppWID  string    `json:"whatsapp_wid,omitempty"`
	OwnerName    string    `json:"owner_name,omitempty"`
	OwnerJID     string    `json:"owner_jid,omitempty"`
	Activated    bool      `json:"activated"`
	Attend       bool      `json:"attend"`
	SyncedAt     time.Time `json:"synced_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewGroupResponse maps a group entity to its DTO.
func NewGroupResponse(g *entity.Group) GroupResponse {
	participants := g.Participants
	if participants == nil {
		participants = []string{}
	}
	admins := g.GroupAdmins
	if admins == nil {
		admins = []string{}
	}
	return GroupResponse{
		ID:           g.ID,
		TenantID:     g.TenantID,
		ChannelID:    g.ChannelID,
		GroupJID:     g.GroupJID,
		Name:         g.Name,
		Description:  g.Description,
		Participants: participants,
		GroupAdmins:  admins,
		CompanyID:    g.CompanyID,
		WhatsAppWID:  g.WhatsAppWID,
		OwnerName:    g.OwnerName,
		OwnerJID:     g.OwnerJID,
		Activated:    g.Activated,
		Attend:       g.Attend,
		SyncedAt:     g.SyncedAt,
		CreatedAt:    g.CreatedAt,
		UpdatedAt:    g.UpdatedAt,
	}
}

// NewGroupResponses maps a slice.
func NewGroupResponses(items []*entity.Group) []GroupResponse {
	out := make([]GroupResponse, len(items))
	for i, g := range items {
		out[i] = NewGroupResponse(g)
	}
	return out
}
