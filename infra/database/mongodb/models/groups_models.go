package models

import "time"

// WhatsAppGroup is the BSON document for a known WhatsApp group (registry synced
// from the gateway). group_jid is unique per tenant. participants/group_admins are
// raw metadata strings — not contacts.
type WhatsAppGroup struct {
	Base         `bson:",inline"`
	ChannelID    string    `bson:"channel_id,omitempty"`
	GroupJID     string    `bson:"group_jid"`
	Name         string    `bson:"name,omitempty"`
	Description  string    `bson:"description,omitempty"`
	Participants []string  `bson:"participants,omitempty"`
	GroupAdmins  []string  `bson:"group_admins,omitempty"`
	CompanyID    string    `bson:"company_id,omitempty"`
	WhatsAppWID  string    `bson:"whatsapp_wid,omitempty"`
	OwnerName    string    `bson:"owner_name,omitempty"`
	OwnerJID     string    `bson:"owner_jid,omitempty"`
	Activated    bool      `bson:"activated"`
	Attend       bool      `bson:"attend"`
	SyncedAt     time.Time `bson:"synced_at"`
}
