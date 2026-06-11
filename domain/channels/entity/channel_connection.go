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
// and never returned to clients (masked). WebhookVerifyToken resolves and
// verifies inbound requests/receipts.
type ChannelConnection struct {
	ID                 string
	TenantID           string
	Type               Type
	Name               string
	Status             Status
	BaseURL            string
	AuthType           AuthType
	Secret             string
	WebhookVerifyToken string
	DefaultSectorID    string
	Enabled            bool
	// AutomationEnabled routes brand-new inbound conversations to the external
	// flow before a human.
	AutomationEnabled bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
