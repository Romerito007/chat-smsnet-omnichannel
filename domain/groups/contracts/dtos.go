// Package contracts holds the groups service inputs and ports.
package contracts

import "context"

// UpsertGroup is one group in a gateway sync batch (the gateway's shape, mapped to
// our field names). GroupJID is required (the idempotency key).
type UpsertGroup struct {
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
}

// ListFilter scopes the group listing for the management screen. Q is a free-text
// search over name + description (text index). Attend, when non-nil, filters by the
// attendance flag.
type ListFilter struct {
	Q         string
	ChannelID string
	Attend    *bool
}

// ChannelEventEmitter delivers an event to a channel's MANAGED webhook (the
// gateway). Implemented by the webhooks dispatcher; used by the group sync to ask
// the gateway to push the group list. Returns an error when the channel has no
// managed webhook (no outbound_url) — the sync cannot work without the gateway.
type ChannelEventEmitter interface {
	EmitToChannel(ctx context.Context, tenantID, channelID, event string, payload any) error
}
