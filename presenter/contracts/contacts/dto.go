// Package contacts holds the response DTOs for the contact read endpoints.
package contacts

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
)

// ExternalID is a contact's identifier on a specific channel.
type ExternalID struct {
	Channel    string `json:"channel"`
	ExternalID string `json:"external_id"`
}

// ContactResponse is the public representation of a contact. Only locally-stored
// fields are returned — never data enriched on demand from a provider.
type ContactResponse struct {
	ID          string       `json:"id"`
	TenantID    string       `json:"tenant_id"`
	Name        string       `json:"name"`
	Phones      []string     `json:"phones"`
	Document    string       `json:"document,omitempty"`
	ExternalIDs []ExternalID `json:"external_ids"`
	Tags        []string     `json:"tags"`
	Notes       string       `json:"notes,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// NewContactResponse maps a contact entity to its DTO. Phones is derived from the
// stored phone (the model keeps a single primary phone today, exposed as a list).
func NewContactResponse(c *entity.Contact) ContactResponse {
	phones := []string{}
	if c.Phone != "" {
		phones = append(phones, c.Phone)
	}
	externalIDs := make([]ExternalID, 0, len(c.Identities))
	for _, id := range c.Identities {
		externalIDs = append(externalIDs, ExternalID{Channel: id.Channel, ExternalID: id.ExternalID})
	}
	tags := c.Tags
	if tags == nil {
		tags = []string{}
	}
	return ContactResponse{
		ID:          c.ID,
		TenantID:    c.TenantID,
		Name:        c.Name,
		Phones:      phones,
		Document:    c.Document,
		ExternalIDs: externalIDs,
		Tags:        tags,
		Notes:       c.Notes,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}
