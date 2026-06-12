// Package entity holds the Contact aggregate.
package entity

import "time"

// ChannelIdentity links a contact to its external identifier on a channel.
type ChannelIdentity struct {
	Channel    string
	ExternalID string
}

// Contact is a person who talks to the operation. Only basic, locally-provided
// fields are stored — never data fetched/enriched from a provider.
type Contact struct {
	ID         string
	TenantID   string
	Name       string
	Phone      string
	Document   string
	Identities []ChannelIdentity
	// Tags are free-form labels applied by agents (CRM-style).
	Tags []string
	// Notes is a free-text agent note about the contact.
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// HasIdentity reports whether the contact already carries the given channel
// identity.
func (c *Contact) HasIdentity(channel, externalID string) bool {
	for _, id := range c.Identities {
		if id.Channel == channel && id.ExternalID == externalID {
			return true
		}
	}
	return false
}
