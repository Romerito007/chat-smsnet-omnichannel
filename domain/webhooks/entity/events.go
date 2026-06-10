// Package entity holds the webhooks domain entities: subscriptions and the
// per-attempt delivery records.
package entity

// Event names a business event a webhook can subscribe to. These are the
// canonical outbound-webhook event names; some mirror realtime event names but
// this set is the webhook contract with external systems.
const (
	EventConversationCreated     = "conversation.created"
	EventConversationAssigned    = "conversation.assigned"
	EventConversationTransferred = "conversation.transferred"
	EventConversationClosed      = "conversation.closed"
	EventMessageCreated          = "message.created"
	EventSLABreached             = "sla.breached"
	EventAutomationCompleted     = "automation.completed"
	EventAutomationFailed        = "automation.failed"
)

// SupportedEvents is the closed set of events a subscription may register for.
// Subscriptions are validated against it so a typo never silently drops events.
var SupportedEvents = []string{
	EventConversationCreated,
	EventConversationAssigned,
	EventConversationTransferred,
	EventConversationClosed,
	EventMessageCreated,
	EventSLABreached,
	EventAutomationCompleted,
	EventAutomationFailed,
}

// IsSupportedEvent reports whether e is a known webhook event.
func IsSupportedEvent(e string) bool {
	for _, s := range SupportedEvents {
		if s == e {
			return true
		}
	}
	return false
}
