// Package notifications holds the request/response DTOs for the notifications
// endpoints.
package notifications

import (
	"time"

	ncontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/contracts"
	nentity "github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/entity"
)

// NotificationResponse is the public representation of a notification.
type NotificationResponse struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Body      string     `json:"body,omitempty"`
	Link      string     `json:"link,omitempty"`
	Read      bool       `json:"read"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
}

// NewNotificationResponse maps an entity.
func NewNotificationResponse(n *nentity.Notification) NotificationResponse {
	return NotificationResponse{
		ID: n.ID, Type: string(n.Type), Title: n.Title, Body: n.Body, Link: n.Link,
		Read: n.Read, CreatedAt: n.CreatedAt, ReadAt: n.ReadAt,
	}
}

// NewNotificationResponses maps a slice.
func NewNotificationResponses(items []*nentity.Notification) []NotificationResponse {
	out := make([]NotificationResponse, 0, len(items))
	for _, n := range items {
		out = append(out, NewNotificationResponse(n))
	}
	return out
}

// PreferencesResponse reports the effective per-type email settings.
type PreferencesResponse struct {
	EmailByType map[string]bool `json:"email_by_type"`
}

// NewPreferencesResponse maps the effective preferences.
func NewPreferencesResponse(p *nentity.Preferences) PreferencesResponse {
	eff := p.Effective()
	out := make(map[string]bool, len(eff))
	for t, v := range eff {
		out[string(t)] = v
	}
	return PreferencesResponse{EmailByType: out}
}

// UpdatePreferencesRequest is the body of PATCH /v1/notifications/preferences.
type UpdatePreferencesRequest struct {
	EmailByType map[string]bool `json:"email_by_type"`
}

// ToCommand maps to the service command.
func (r UpdatePreferencesRequest) ToCommand() ncontracts.UpdatePreferences {
	byType := make(map[nentity.Type]bool, len(r.EmailByType))
	for t, v := range r.EmailByType {
		byType[nentity.Type(t)] = v
	}
	return ncontracts.UpdatePreferences{EmailByType: byType}
}
