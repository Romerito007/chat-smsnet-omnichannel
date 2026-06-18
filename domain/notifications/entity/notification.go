// Package entity holds the notifications domain entities: notifications and
// per-user email preferences.
package entity

import "time"

// Type identifies a notification's kind. It also drives the default email
// eligibility.
type Type string

const (
	TypeAssignedToYou    Type = "conversation.assigned_to_you"
	TypeTransferredToYou Type = "conversation.transferred_to_you"
	TypeMention          Type = "mention.internal_note"
	TypeSLAAtRisk        Type = "sla.at_risk"
	TypeSLABreached      Type = "sla.breached"
	TypeChannelError     Type = "channel.connection_error"
	// TypeChannelTemplatesUpdated alerts agents that a channel's WhatsApp template
	// mirror was refreshed (pushed by the gateway).
	TypeChannelTemplatesUpdated Type = "channel.templates_updated"
)

// AllTypes is the closed set of notification types.
var AllTypes = []Type{
	TypeAssignedToYou, TypeTransferredToYou, TypeMention,
	TypeSLAAtRisk, TypeSLABreached, TypeChannelError, TypeChannelTemplatesUpdated,
}

// IsValidType reports whether t is a known type.
func IsValidType(t Type) bool {
	for _, x := range AllTypes {
		if x == t {
			return true
		}
	}
	return false
}

// DefaultEmail reports whether a type emails by default (before any per-user
// preference). Operationally important alerts default on; routine in-app ones
// default off.
func DefaultEmail(t Type) bool {
	switch t {
	case TypeSLAAtRisk, TypeSLABreached, TypeChannelError:
		return true
	default:
		return false
	}
}

// Notification is one in-app notification for a recipient user.
type Notification struct {
	ID        string
	TenantID  string
	UserID    string
	Type      Type
	Title     string
	Body      string
	Link      string
	Read      bool
	CreatedAt time.Time
	ReadAt    *time.Time
}
