// Package contracts holds the contact service inputs.
package contracts

import "context"

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
	Name        string
	Phones      []string
	Document    string
	Email       string
	ExternalIDs []ExternalIdentity
	Tags        []string
	Notes       string
}

// UpdateContact is the partial input to edit a contact. Nil pointers leave the
// field unchanged.
type UpdateContact struct {
	Name        *string
	Phones      *[]string
	Document    *string
	Email       *string
	Tags        *[]string
	Notes       *string
	ExternalIDs *[]ExternalIdentity
}

// TagResolver maps tag refs (id or name) to canonical ids, so contact tags are
// normalized like conversation tags. It is implemented by the conversationtools
// tag service. Lenient resolution (strict=false) keeps free-text labels that
// don't match any catalog tag. Optional: when unset, tags are stored as-is.
type TagResolver interface {
	ResolveTags(ctx context.Context, refs []string, strict bool) ([]string, error)
}
