package shared

import "context"

// RuleEventSink receives conversation/message lifecycle events for asynchronous
// automation-rule evaluation. It is implemented by the automation-rules enqueuer
// (which puts a task on Asynq) so the emitting services never block on rule work.
// event is the internal dot-notation name (e.g. "conversation.created").
type RuleEventSink interface {
	EmitRuleEvent(ctx context.Context, tenantID, event, conversationID string, payload any)
}

// RuleOrigin tags what caused a lifecycle event, carried on the context so the
// emitting service does not need a wider signature. It is the PRIMARY anti-loop
// mechanism: events produced by an automation action are tagged OriginAutomation,
// and the automation evaluator never fires rules from automation-origin events —
// so automation cannot feed itself.
type RuleOrigin string

const (
	// OriginExternal is the default: a human/customer/integration-caused event.
	OriginExternal RuleOrigin = "external"
	// OriginAutomation marks an event produced by an automation rule action.
	OriginAutomation RuleOrigin = "automation"
)

type ruleOriginKey struct{}

// WithRuleOrigin returns a context tagged with the given rule-event origin. The
// automation executor wraps its action calls with OriginAutomation so every
// lifecycle event they emit is suppressed by the evaluator.
func WithRuleOrigin(ctx context.Context, o RuleOrigin) context.Context {
	return context.WithValue(ctx, ruleOriginKey{}, o)
}

// RuleOriginFromContext returns the context's rule origin, defaulting to
// OriginExternal when unset.
func RuleOriginFromContext(ctx context.Context) RuleOrigin {
	if o, ok := ctx.Value(ruleOriginKey{}).(RuleOrigin); ok && o != "" {
		return o
	}
	return OriginExternal
}

type ruleDepthKey struct{}

// WithRuleDepth returns a context carrying the causal depth of the chain that is
// producing events. The automation executor increments it per action; the
// evaluator caps the chain length (defense-in-depth anti-loop layer 3).
func WithRuleDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, ruleDepthKey{}, depth)
}

// RuleDepthFromContext returns the context's causal depth (0 when unset).
func RuleDepthFromContext(ctx context.Context) int {
	if d, ok := ctx.Value(ruleDepthKey{}).(int); ok {
		return d
	}
	return 0
}
