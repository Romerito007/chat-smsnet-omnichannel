package entity

// Behavior is the per-assistant runtime behavior the copilot applies to one
// inference: the customer-data privacy gate (the only data section pre-injected
// into the prompt — financial/monitoring are consulted on demand via ISP tools,
// not pre-injected), the human-approval switch for write tools, the sampling
// temperature, the max output tokens, and free-text system instructions
// (persona/conduct). It is resolved from the conversation's CopilotAssistant; when
// no assistant resolves, DefaultBehavior is used — conservative (gate off, no
// persona, default sampling).
type Behavior struct {
	AllowCustomerData     bool
	HumanApprovalRequired bool
	// WriteModes carries the assistant's per-write-operation modes (ISP action slug
	// → automatico|mediante_aprovacao) so the agentic loop can decide, per write,
	// whether to execute it automatically or propose it for approval.
	WriteModes         map[string]string
	Temperature        float64
	MaxTokens          int
	SystemInstructions string
}

// WriteModeFor returns the execution mode for a write operation (by ISP action
// slug). Anything not explicitly set to "automatico" — including an unknown action
// or an empty map — resolves to approval, so automatic execution is strictly
// opt-in per operation.
func (b Behavior) WriteModeFor(action string) string {
	if action != "" && b.WriteModes[action] == WriteModeAuto {
		return WriteModeAuto
	}
	return WriteModeApproval
}

// Default sampling for an assistant / the no-assistant fallback.
const (
	DefaultTemperature = 0.7
	DefaultMaxTokens   = 512
)

// DefaultBehavior is the conservative behavior used when a conversation resolves to
// no assistant (empty channel_id, or no assistant serving it): the customer-data
// gate is OFF, no persona, default sampling.
func DefaultBehavior() Behavior {
	return Behavior{
		Temperature: DefaultTemperature,
		MaxTokens:   DefaultMaxTokens,
	}
}

// Behavior returns the assistant's runtime behavior.
func (a *Assistant) Behavior() Behavior {
	return Behavior{
		AllowCustomerData:     a.AllowCustomerData,
		HumanApprovalRequired: a.HumanApprovalRequired,
		WriteModes:            a.WriteModes,
		Temperature:           a.Temperature,
		MaxTokens:             a.MaxTokens,
		SystemInstructions:    a.SystemInstructions,
	}
}
