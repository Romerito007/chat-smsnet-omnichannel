package entity

import "time"

// Event types recorded on a conversation timeline.
const (
	EventConversationCreated     = "conversation.created"
	EventMessageCreated          = "message.created"
	EventInternalNoteAdded       = "internal_note.added"
	EventConversationUpdated     = "conversation.updated"
	EventConversationClosed      = "conversation.closed"
	EventConversationReopened    = "conversation.reopened"
	EventConversationAssigned    = "conversation.assigned"
	EventConversationTransferred = "conversation.transferred"
	EventConversationEnqueued    = "conversation.enqueued"
)

// ActorType identifies who triggered a conversation event.
type ActorType string

const (
	ActorAgent      ActorType = "agent"
	ActorCustomer   ActorType = "customer"
	ActorSystem     ActorType = "system"
	ActorAutomation ActorType = "automation"
	ActorCopilot    ActorType = "copilot"
)

// ConversationEvent is an immutable audit/timeline record of something that
// happened on a conversation.
type ConversationEvent struct {
	ID             string
	TenantID       string
	ConversationID string
	Type           string
	ActorType      ActorType
	ActorID        string
	Data           map[string]any
	CreatedAt      time.Time
}
