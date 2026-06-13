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
	ID       string
	TenantID string
	Name     string
	// Phone is the primary phone (== Phones[0]), kept denormalized for the inbound
	// upsert, search and dedup paths.
	Phone string
	// Phones are all of the contact's phone numbers (CRM).
	Phones     []string
	Document   string
	Email      string
	Identities []ChannelIdentity
	// Tags are CRM labels. Catalog tags are normalized to their canonical IDs
	// (names resolved server-side, mirroring conversation tags); free-text labels
	// that match no catalog tag are kept as-is.
	Tags []string
	// Notes is a free-text agent note about the contact.
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SetPhones replaces the phone list (already normalized by the caller) and keeps
// the primary Phone in sync as the first entry.
func (c *Contact) SetPhones(phones []string) {
	c.Phones = phones
	if len(phones) > 0 {
		c.Phone = phones[0]
	} else {
		c.Phone = ""
	}
}

// AddPhone appends a phone if not already present, keeping the primary in sync.
func (c *Contact) AddPhone(phone string) {
	if phone == "" {
		return
	}
	for _, p := range c.Phones {
		if p == phone {
			return
		}
	}
	c.SetPhones(append(c.Phones, phone))
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
