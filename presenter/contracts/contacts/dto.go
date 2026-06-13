// Package contacts holds the request/response DTOs for the contact endpoints.
package contacts

import (
	"time"

	ccontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
)

// ExternalID is a contact's identifier on a specific channel.
type ExternalID struct {
	Channel    string `json:"channel"`
	ExternalID string `json:"external_id"`
}

// ── requests ─────────────────────────────────────────────────────────────────

// CreateContactRequest is the body of POST /v1/contacts.
type CreateContactRequest struct {
	Name               string       `json:"name"`
	Phones             []string     `json:"phones"`
	Document           string       `json:"document"`
	Email              string       `json:"email"`
	ExternalIDs        []ExternalID `json:"external_ids"`
	Tags               []string     `json:"tags"`
	Notes              string       `json:"notes"`
	AvatarAttachmentID string       `json:"avatar_attachment_id"`
}

// ToCommand maps the request to the service command.
func (r CreateContactRequest) ToCommand() ccontracts.CreateContact {
	return ccontracts.CreateContact{
		Name:               r.Name,
		Phones:             r.Phones,
		Document:           r.Document,
		Email:              r.Email,
		ExternalIDs:        toExternalIdentities(r.ExternalIDs),
		Tags:               r.Tags,
		Notes:              r.Notes,
		AvatarAttachmentID: r.AvatarAttachmentID,
	}
}

// UpdateContactRequest is the body of PATCH /v1/contacts/{id}. Nil fields are
// left unchanged.
type UpdateContactRequest struct {
	Name               *string       `json:"name"`
	Phones             *[]string     `json:"phones"`
	Document           *string       `json:"document"`
	Email              *string       `json:"email"`
	Tags               *[]string     `json:"tags"`
	Notes              *string       `json:"notes"`
	ExternalIDs        *[]ExternalID `json:"external_ids"`
	AvatarAttachmentID *string       `json:"avatar_attachment_id"`
}

// ToCommand maps the request to the service command.
func (r UpdateContactRequest) ToCommand() ccontracts.UpdateContact {
	cmd := ccontracts.UpdateContact{
		Name:               r.Name,
		Phones:             r.Phones,
		Document:           r.Document,
		Email:              r.Email,
		Tags:               r.Tags,
		Notes:              r.Notes,
		AvatarAttachmentID: r.AvatarAttachmentID,
	}
	if r.ExternalIDs != nil {
		ids := toExternalIdentities(*r.ExternalIDs)
		cmd.ExternalIDs = &ids
	}
	return cmd
}

func toExternalIdentities(ids []ExternalID) []ccontracts.ExternalIdentity {
	out := make([]ccontracts.ExternalIdentity, 0, len(ids))
	for _, id := range ids {
		out = append(out, ccontracts.ExternalIdentity{Channel: id.Channel, ExternalID: id.ExternalID})
	}
	return out
}

// ── responses ────────────────────────────────────────────────────────────────

// ContactResponse is the public representation of a contact. Only locally-stored
// fields are returned — never data enriched on demand from a provider.
type ContactResponse struct {
	ID                 string       `json:"id"`
	TenantID           string       `json:"tenant_id"`
	Name               string       `json:"name"`
	Phones             []string     `json:"phones"`
	Document           string       `json:"document,omitempty"`
	Email              string       `json:"email,omitempty"`
	ExternalIDs        []ExternalID `json:"external_ids"`
	Tags               []string     `json:"tags"`
	Notes              string       `json:"notes,omitempty"`
	AvatarAttachmentID string       `json:"avatar_attachment_id,omitempty"`
	// AvatarURL is a short-lived signed URL the browser loads directly (no JWT).
	// Read-only/derived; present only when the avatar exists and is ready.
	AvatarURL string    `json:"avatar_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewContactResponse maps a contact entity to its DTO. Phones falls back to the
// primary phone for older records stored before the phones list existed.
func NewContactResponse(c *entity.Contact) ContactResponse {
	phones := c.Phones
	if len(phones) == 0 && c.Phone != "" {
		phones = []string{c.Phone}
	}
	if phones == nil {
		phones = []string{}
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
		ID:                 c.ID,
		TenantID:           c.TenantID,
		Name:               c.Name,
		Phones:             phones,
		Document:           c.Document,
		Email:              c.Email,
		ExternalIDs:        externalIDs,
		Tags:               tags,
		Notes:              c.Notes,
		AvatarAttachmentID: c.AvatarAttachmentID,
		CreatedAt:          c.CreatedAt,
		UpdatedAt:          c.UpdatedAt,
	}
}

// NewContactResponses maps a slice.
func NewContactResponses(items []*entity.Contact) []ContactResponse {
	return NewContactResponsesWithAvatars(items, nil)
}

// NewContactResponsesWithAvatars maps a slice, attaching each contact's signed
// avatar URL from avatarURLs (keyed by avatar attachment id; resolved in batch).
func NewContactResponsesWithAvatars(items []*entity.Contact, avatarURLs map[string]string) []ContactResponse {
	out := make([]ContactResponse, len(items))
	for i, c := range items {
		r := NewContactResponse(c)
		if c.AvatarAttachmentID != "" {
			r.AvatarURL = avatarURLs[c.AvatarAttachmentID]
		}
		out[i] = r
	}
	return out
}

// NewContactResponseWithAvatar maps one contact, attaching its signed avatar URL.
func NewContactResponseWithAvatar(c *entity.Contact, avatarURLs map[string]string) ContactResponse {
	r := NewContactResponse(c)
	if c.AvatarAttachmentID != "" {
		r.AvatarURL = avatarURLs[c.AvatarAttachmentID]
	}
	return r
}
