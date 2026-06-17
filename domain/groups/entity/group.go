// Package entity holds the WhatsApp group aggregate. A group is a lightweight
// registry record synced from the gateway — NOT a contact/conversation. The
// contact-of-type-group + conversation are created on demand on the first message
// (Domain 2), and only when Attend is true.
package entity

import "time"

// Group is a known WhatsApp group for a tenant, populated by the gateway sync.
// GroupJID ("120363...@g.us") is unique per tenant. Participants/GroupAdmins are
// raw metadata strings (phones/JIDs) — never normalized or turned into contacts;
// routing is by JID, not phone.
type Group struct {
	ID           string
	TenantID     string
	ChannelID    string
	GroupJID     string
	Name         string
	Description  string
	Participants []string
	GroupAdmins  []string
	CompanyID    string
	WhatsAppWID  string
	OwnerName    string
	OwnerJID     string
	Activated    bool
	// Attend is the attendance filter: true (default) means the group is attended —
	// its first message creates a conversation (Domain 2). false means the tenant
	// opted out: messages are dropped without creating a conversation. The sync
	// NEVER resets this (it is the operator's choice).
	Attend    bool
	SyncedAt  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}
