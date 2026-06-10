// Package contracts holds the routing service inputs/outputs.
package contracts

// AssignCommand assigns a conversation to a specific agent (manual).
type AssignCommand struct {
	AgentID string
}

// TransferCommand moves a conversation to another sector and/or agent.
type TransferCommand struct {
	SectorID string
	AgentID  string
}

// EnqueueCommand places a conversation into a queue.
type EnqueueCommand struct {
	QueueID string
}

// RunCommand triggers automatic routing. When ConversationID is set only that
// conversation is routed; otherwise a batch of waiting conversations is routed.
type RunCommand struct {
	ConversationID string
}

// AssignmentResult records a successful auto-assignment.
type AssignmentResult struct {
	ConversationID string `json:"conversation_id"`
	AgentID        string `json:"agent_id"`
}

// SkippedResult records a conversation that could not be routed and why.
type SkippedResult struct {
	ConversationID string `json:"conversation_id"`
	Reason         string `json:"reason"`
}

// RunResult is the outcome of a routing run.
type RunResult struct {
	Assigned []AssignmentResult `json:"assigned"`
	Skipped  []SkippedResult    `json:"skipped"`
}
