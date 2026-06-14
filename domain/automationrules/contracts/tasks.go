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
}
