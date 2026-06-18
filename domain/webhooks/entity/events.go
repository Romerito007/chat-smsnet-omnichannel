// Package entity holds the webhooks domain entities: subscriptions and the
// per-attempt delivery records.
package entity

// Event names the wire event a webhook subscribes to and that is delivered. The
// vocabulary follows Chatwoot's underscore convention (form parity) so a system
// already consuming Chatwoot webhooks parses ours with minimal change: names that
// Chatwoot has are identical (conversation_created, conversation_status_changed,
// message_created); the rest are our additions in the same convention.
const (
	EventConversationCreated       = "conversation_created"        // Chatwoot ✓
	EventConversationStatusChanged = "conversation_status_changed" // Chatwoot ✓ (fired on close)
	EventConversationAssigned      = "conversation_assigned"       // ours
	EventConversationTransferred   = "conversation_transferred"    // ours
	EventConversationUpdated       = "conversation_updated"        // Chatwoot ✓
	EventMessageCreated            = "message_created"             // Chatwoot ✓
	EventMessageUpdated            = "message_updated"             // Chatwoot ✓ (edit)
	EventContactCreated            = "contact_created"             // Chatwoot ✓
	EventContactUpdated            = "contact_updated"             // Chatwoot ✓
	EventSLABreached               = "sla_breached"                // ours
)

// Channel-control events are delivered ONLY to a channel's managed webhook (via
// Dispatcher.EmitToChannel), asking the gateway to push data back. They are
// intentionally kept OUT of the public SupportedEvents catalog.
const (
	EventGroupSyncRequested     = "group_sync_requested"
	EventTemplatesSyncRequested = "templates_sync_requested"
)

// SupportedEvents is the closed set of wire events a subscription may register
// for. Subscriptions are validated against it so a typo never silently drops events.
var SupportedEvents = []string{
	EventConversationCreated,
	EventConversationStatusChanged,
	EventConversationAssigned,
	EventConversationTransferred,
	EventConversationUpdated,
	EventMessageCreated,
	EventMessageUpdated,
	EventContactCreated,
	EventContactUpdated,
	EventSLABreached,
}

// internalToWire maps the internal event keys emitted by domain services (dot
// convention, shared with the realtime/WS layer) to the Chatwoot-style underscore
// WIRE names. Keeping this mapping in the webhooks domain lets the webhook contract
// stay aligned to Chatwoot without touching the emitters or the realtime vocabulary.
var internalToWire = map[string]string{
	"conversation.created":     EventConversationCreated,
	"conversation.closed":      EventConversationStatusChanged,
	"conversation.assigned":    EventConversationAssigned,
	"conversation.transferred": EventConversationTransferred,
	"conversation.updated":     EventConversationUpdated,
	"message.created":          EventMessageCreated,
	"message.updated":          EventMessageUpdated,
	"contact.created":          EventContactCreated,
	"contact.updated":          EventContactUpdated,
	"sla.breached":             EventSLABreached,
}

// WireEvent maps an internal event key to its webhook wire name; ok is false when
// the event is not part of the webhook contract.
func WireEvent(internal string) (string, bool) {
	w, ok := internalToWire[internal]
	return w, ok
}

// IsSupportedEvent reports whether e is a known wire webhook event.
func IsSupportedEvent(e string) bool {
	for _, s := range SupportedEvents {
		if s == e {
			return true
		}
	}
	return false
}
