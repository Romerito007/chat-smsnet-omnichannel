package contracts

import "context"

// ToolBroker discovers the tenant's external tools (via MCP) for a conversation
// and yields a session that can execute read tools and propose write tools. The
// copilot depends only on this port — it never knows any concrete tool or the
// MCP transport. Implemented by the mcp domain.
type ToolBroker interface {
	OpenToolSession(ctx context.Context, conversationID string) (ToolSession, error)
}

// ToolSession is the per-conversation tool context for one agentic run. It owns
// the mapping from tool name to its server, so the copilot can stay name-agnostic.
type ToolSession interface {
	// Tools are the tool definitions offered to the model (read tools are
	// callable; write tools are marked not-read-only and only ever proposed).
	Tools() []ToolDefinition
	// IsWrite reports whether the named tool has side effects.
	IsWrite(name string) bool
	// WriteAction returns the ISP action slug a write tool maps to (e.g.
	// "liberacao"/"chamado"), or "" for a write not in the explicit SMSNET table.
	// The copilot uses it to resolve the assistant's per-operation mode; an empty
	// slug always falls back to approval.
	WriteAction(name string) string
	// ExecuteRead runs a read tool and returns its result text.
	ExecuteRead(ctx context.Context, name, argsJSON string) (string, error)
	// ExecuteWrite runs a write tool directly (no approval) and returns its result.
	// Used only when the assistant set the operation to "automatico"; it audits the
	// automatic execution.
	ExecuteWrite(ctx context.Context, name, argsJSON string) (string, error)
	// ProposeWrite records a write tool call as a pending approval (never
	// executing it) and returns the confirmation card.
	ProposeWrite(ctx context.Context, name, argsJSON string) (ProposedAction, error)
}

// ProposedAction is a write tool the model proposed; it awaits explicit human
// approval before execution. The args are non-secret and shown on the card.
type ProposedAction struct {
	ApprovalID string         `json:"approval_id"`
	Server     string         `json:"server"`
	Tool       string         `json:"tool"`
	Args       map[string]any `json:"args"`
}
