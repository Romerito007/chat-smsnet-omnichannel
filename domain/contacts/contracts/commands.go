// Package contracts holds the contact service inputs.
package contracts

import "context"

// ListFilter narrows a contact listing. All fields are optional and combine with
// AND (and with the free-text Query). Empty fields are ignored; tenant scope is
// applied separately by the repository. Kept deliberately lean for the pilot:
// substring match on name/phone, exact membership on tag id — no operators yet.
type ListFilter struct {
	// Query is the free-text search (?q=): case-insensitive substring over
	// name/phone/document/email.
	Query string
	// Name filters by case-insensitive substring of the contact name.
	Name string
	// Phone filters by substring of any phone in the contact's phones.
	Phone string
	// TagID keeps only contacts that carry this tag id (tags are stored as ids).
	TagID string
}

// UpsertFromInbound carries the basic, locally-provided contact fields extracted
// from an inbound channel message. No provider enrichment is performed.
type UpsertFromInbound struct {
	Channel    string
	ExternalID string
	Name       string
	Phone      string
	Document   string
}

// ExternalIdentity is a contact's id on a channel (CRM input).
type ExternalIdentity struct {
	Channel    string
	ExternalID string
}

// CreateContact is the input to create a CRM contact. Only locally-provided
// fields are stored — never provider-enriched data.
type CreateContact struct {
	Name               string
	Phones             []string
	Document           string
	Email              string
	ExternalIDs        []ExternalIdentity
	Tags               []string
	Notes              string
	AvatarAttachmentID string
}

// UpdateContact is the partial input to edit a contact. Nil pointers leave the
// field unchanged.
type UpdateContact struct {
	Name               *string
	Phones             *[]string
	Document           *string
	Email              *string
	Tags               *[]string
	Notes              *string
	ExternalIDs        *[]ExternalIdentity
	AvatarAttachmentID *string
	// CustomAttributes, when non-nil, replaces the whole custom-attributes map
	// (omit a key to remove it). Validated against applies_to=contact definitions.
	CustomAttributes *map[string]any
}

// AvatarValidator validates that an attachment id may be used as a contact
// avatar: it must exist in the same tenant, be an image, and be ready. Returns a
// validation error otherwise. Implemented by the attachments service. Optional:
// when unset, the avatar id is stored without validation (mirrors User avatar).
type AvatarValidator interface {
	ValidateReadyImage(ctx context.Context, attachmentID string) error
}

// TagResolver maps tag refs (id or name) to canonical ids, so contact tags are
// normalized like conversation tags. It is implemented by the conversationtools
// tag service. Lenient resolution (strict=false) keeps free-text labels that
// don't match any catalog tag. Optional: when unset, tags are stored as-is.
type TagResolver interface {
	ResolveTags(ctx context.Context, refs []string, strict bool) ([]string, error)
}
