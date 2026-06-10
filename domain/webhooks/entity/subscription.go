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
	CreatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

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
