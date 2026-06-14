package contracts

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
)

// Internal event keys emitted by the contacts service (dot convention; the
// webhooks domain maps them to the Chatwoot-style wire names contact_created /
// contact_updated).
const (
	EventContactCreated = "contact.created"
	EventContactUpdated = "contact.updated"
)

// ContactIdentity mirrors a contact's external channel identity on the wire.
type ContactIdentity struct {
	Channel    string `json:"channel"`
	ExternalID string `json:"external_id"`
}

// ContactPayload is the event representation of a contact, delivered to webhooks
// on contact_created / contact_updated.
type ContactPayload struct {
	ID         string            `json:"id"`
	TenantID   string            `json:"tenant_id"`
	Name       string            `json:"name"`
	Phone      string            `json:"phone,omitempty"`
	Phones     []string          `json:"phones,omitempty"`
	Document   string            `json:"document,omitempty"`
	Email      string            `json:"email,omitempty"`
	Identities []ContactIdentity `json:"identities,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// NewContactPayload builds the payload from a contact entity.
func NewContactPayload(c *entity.Contact) ContactPayload {
	ids := make([]ContactIdentity, len(c.Identities))
	for i, id := range c.Identities {
		ids[i] = ContactIdentity{Channel: id.Channel, ExternalID: id.ExternalID}
	}
	return ContactPayload{
		ID:         c.ID,
		TenantID:   c.TenantID,
		Name:       c.Name,
		Phone:      c.Phone,
		Phones:     c.Phones,
		Document:   c.Document,
		Email:      c.Email,
		Identities: ids,
		Tags:       c.Tags,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}
