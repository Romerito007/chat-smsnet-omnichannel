// Package contracts holds the contact service inputs.
package contracts

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
