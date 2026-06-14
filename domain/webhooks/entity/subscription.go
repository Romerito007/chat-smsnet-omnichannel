package entity

import "time"

// WebhookSubscription is a tenant's registered outbound webhook endpoint. The
// Secret is held in memory only (plaintext) and stored encrypted at rest; it is
// used to sign deliveries with HMAC-SHA256 and is never returned after creation.
type WebhookSubscription struct {
	ID              string
	TenantID        string
	Name            string
	URL             string
	Events          []string
	Scopes          []string
	Secret          string
	Enabled         bool
	RateLimitPerMin int
	// OwnedByChannelID, when set, marks this subscription as MANAGED by a channel
	// connection (its URL/secret/events are kept in sync by the channel and it
	// cannot be edited or deleted directly through the webhooks API). Empty for a
	// normal, manually-created subscription.
	OwnedByChannelID string
	CreatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Managed reports whether the subscription is owned/kept in sync by a channel
// connection (so the webhooks API must refuse to edit or delete it directly).
func (s *WebhookSubscription) Managed() bool { return s.OwnedByChannelID != "" }

// SubscribesTo reports whether the subscription is enabled and listens for the
// given event.
func (s *WebhookSubscription) SubscribesTo(event string) bool {
	if !s.Enabled {
		return false
	}
	for _, e := range s.Events {
		if e == event {
			return true
		}
	}
	return false
}
