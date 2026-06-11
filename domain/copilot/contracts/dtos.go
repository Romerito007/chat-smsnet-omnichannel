package contracts

import "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"

// RealtimeSuggestionCompleted is the realtime event emitted when a copilot call
// finishes, so the requesting agent's UI can render the result.
const RealtimeSuggestionCompleted = "copilot.suggestion_completed"

// SaveConfig is the input to create-or-update the tenant's copilot config. Nil
// pointers leave existing values unchanged (PATCH semantics).
type SaveConfig struct {
	Provider              *string
	Model                 *string
	APIKey                *string
	BaseURL               *string
	Temperature           *float64
	MaxTokens             *int
	AllowCustomerData     *bool
	AllowFinancialData    *bool
	AllowMonitoringData   *bool
	HumanApprovalRequired *bool
	Enabled               *bool
}

// SuggestReplyInput requests a drafted reply for a conversation.
type SuggestReplyInput struct {
	ConversationID string
	Instruction    string
}

// SummarizeInput requests a conversation summary.
type SummarizeInput struct {
	ConversationID string
}

// ClassifyInput requests classification of a conversation into one of the given
// categories.
type ClassifyInput struct {
	ConversationID string
	Categories     []string
}

// NextActionInput requests a recommended next action for a conversation.
type NextActionInput struct {
	ConversationID string
}

// Result is the normalized copilot output returned to the caller and published
// over realtime.
type Result struct {
	Action           entity.Action `json:"action"`
	Provider         string        `json:"provider"`
	Model            string        `json:"model"`
	Text             string        `json:"text,omitempty"`
	Categories       []string      `json:"categories,omitempty"`
	TokensInput      int           `json:"tokens_input"`
	TokensOutput     int           `json:"tokens_output"`
	EstimatedCost    float64       `json:"estimated_cost"`
	RequiresApproval bool          `json:"requires_approval"`
}
