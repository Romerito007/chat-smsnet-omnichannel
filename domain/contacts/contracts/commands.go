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
