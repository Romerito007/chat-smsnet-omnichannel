package contracts

import "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"

// RealtimeSuggestionCompleted is the realtime event emitted when a copilot call
// finishes, so the requesting agent's UI can render the result.
const RealtimeSuggestionCompleted = "copilot.suggestion_completed"

// SaveConfig is the input to create-or-update the tenant's copilot AI infra. Nil
// pointers leave existing values unchanged (PATCH semantics). Behavior (gates,
// sampling, persona) is set per assistant, not here.
type SaveConfig struct {
	Provider *string
	Model    *string
	APIKey   *string
	BaseURL  *string
	Enabled  *bool
}

// SuggestReplyInput requests a drafted reply for a conversation.
type SuggestReplyInput struct {
	ConversationID string
	Instruction    string
}

// AgentChatInput is one turn of the AGENT↔assistant side chat: the agent's new
// question plus the prior turns (front-managed, ephemeral). The backend is
// stateless — it does not persist the history; it bounds it to the last few turns.
type AgentChatInput struct {
	ConversationID string
	Instruction    string      // the agent's new question
	History        []AgentTurn // prior agent↔assistant turns (oldest first)
}

// AgentTurn is one message of the agent↔assistant side chat. Role is "agent" or
// "assistant"; anything else is treated as "agent".
type AgentTurn struct {
	Role string
	Text string
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
	// ProposedActions are write tools the model proposed during the agentic loop.
	// They are NEVER executed automatically — each awaits explicit agent approval.
	ProposedActions []ProposedAction `json:"proposed_actions,omitempty"`
}
