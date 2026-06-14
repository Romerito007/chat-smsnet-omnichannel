package entity

// Behavior is the per-assistant runtime behavior the copilot applies to one
// inference: the privacy gates (which data sections may enter the prompt), the
// human-approval switch for write tools, the sampling temperature, the max output
// tokens, and free-text system instructions (persona/conduct). It is resolved from
// the conversation's CopilotAssistant; when no assistant resolves, DefaultBehavior
// is used — conservative (all gates off, no persona, default sampling).
type Behavior struct {
	AllowCustomerData     bool
	AllowFinancialData    bool
	AllowMonitoringData   bool
	HumanApprovalRequired bool
	Temperature           float64
	MaxTokens             int
	SystemInstructions    string
}

// Default sampling for an assistant / the no-assistant fallback.
const (
	DefaultTemperature = 0.7
	DefaultMaxTokens   = 512
)

// DefaultBehavior is the conservative behavior used when a conversation resolves to
// no assistant (empty channel_id, or no assistant serving it): every data gate is
// OFF (nothing sensitive reaches the provider), no persona, default sampling.
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
		AllowFinancialData:    a.AllowFinancialData,
		AllowMonitoringData:   a.AllowMonitoringData,
		HumanApprovalRequired: a.HumanApprovalRequired,
		Temperature:           a.Temperature,
		MaxTokens:             a.MaxTokens,
		SystemInstructions:    a.SystemInstructions,
	}
}
