// Package entity holds the channels domain aggregates: ChannelConnection (the
// channel configuration/credential), OutboundDelivery and the InboundRecord
// idempotency ledger.
package entity

import "time"

// Type is the channel kind.
type Type string

const (
	// TypeAPI is the generic HTTP "API channel": any external system integrates
	// by POSTing inbound messages in and receiving outbound deliveries on its
	// configured webhook (outbound_url), both HMAC-signed.
	TypeAPI       Type = "api"
	TypeWhatsApp  Type = "whatsapp"
	TypeTelegram  Type = "telegram"
	TypeInstagram Type = "instagram"
	TypeWebchat   Type = "webchat"
	TypeCustom    Type = "custom"
)

// Valid reports whether t is a known channel type.
func (t Type) Valid() bool {
	switch t {
	case TypeAPI, TypeWhatsApp, TypeTelegram, TypeInstagram, TypeWebchat, TypeCustom:
		return true
	}
	return false
}

// Status is the connection's operational state.
type Status string

const (
	StatusConnected    Status = "connected"
	StatusDisconnected Status = "disconnected"
	StatusError        Status = "error"
)

// AuthType describes how the channel authenticates outbound/inbound traffic.
type AuthType string

const (
	AuthNone  AuthType = "none"
	AuthToken AuthType = "token" // exact secret/bearer
	AuthHMAC  AuthType = "hmac"  // HMAC-SHA256 of the body
)

// ChannelConnection is a per-tenant channel configuration. Secret is the
// channel credential, held in plaintext in memory but stored encrypted at rest
// and never returned to clients (masked).
//
// The integration token resolves and authenticates inbound requests/receipts
// without the front's Bearer/JWT (Chatwoot api_access_token style). It is split:
//   - InboundToken is the high-entropy plaintext, set only on create/rotate and
//     revealed to the client exactly once; it is never persisted nor loaded back.
//   - InboundTokenHash is the SHA-256 hex of the token, the only form stored at
//     rest and the one inbound lookups/comparisons use.
type ChannelConnection struct {
	ID               string
	TenantID         string
	Type             Type
	Name             string
	Status           Status
	BaseURL          string
	AuthType         AuthType
	Secret           string
	InboundToken     string // transient plaintext (create/rotate only; never persisted)
	InboundTokenHash string // SHA-256 hex stored at rest
	DefaultSectorID  string
	// BusinessHours is the channel's free-form weekly schedule + timezone (parsed
	// by businesshours/entity.ParseSchedule). Empty/absent = 24/7.
	BusinessHours map[string]any
	Enabled       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
