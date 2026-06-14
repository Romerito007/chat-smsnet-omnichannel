// Package contracts holds the automation-rules async task payloads and ports.
package contracts

import "encoding/json"

// EvaluateTask is the Asynq payload for automationrule.evaluate. It carries the
// internal event, the conversation to hydrate, and the original event payload
// (used as the webhook data). No conversation/contact fields travel here — the
// worker hydrates the live conversation + contact for condition matching.
type EvaluateTask struct {
	TenantID       string          `json:"tenant_id"`
	Event          string          `json:"event"` // internal dot-notation, e.g. "conversation.created"
	ConversationID string          `json:"conversation_id"`
	Data           json.RawMessage `json:"data"` // original event payload, forwarded as the webhook data
	// EventID uniquely identifies this event occurrence. The evaluator dedups on
	// (rule, event_id) and CLAIMS the key before running actions, so an Asynq retry
	// of the same task never re-runs side-effectful actions (e.g. send_message).
	EventID string `json:"event_id"`
	// Origin is the anti-loop tag: "automation" events were produced by a rule
	// action and the evaluator skips them entirely (default "external").
	Origin string `json:"origin"`
}
