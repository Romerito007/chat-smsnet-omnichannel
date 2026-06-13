package shared

import "context"

// RuleEventSink receives conversation/message lifecycle events for asynchronous
// automation-rule evaluation. It is implemented by the automation-rules enqueuer
// (which puts a task on Asynq) so the emitting services never block on rule work.
// event is the internal dot-notation name (e.g. "conversation.created").
type RuleEventSink interface {
	EmitRuleEvent(ctx context.Context, tenantID, event, conversationID string, payload any)
}
