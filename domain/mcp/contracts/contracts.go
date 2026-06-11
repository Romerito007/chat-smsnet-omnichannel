// Package contracts holds the MCP domain ports (the transport-agnostic MCP
// client) and the service command/result types.
package contracts

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
)

// RealtimeApprovalRequested is published to the conversation/agent when the
// copilot proposes a write action that needs confirmation.
const RealtimeApprovalRequested = "mcp.approval_requested"

// ToolSpec is a tool as reported by an MCP server's tools/list.
type ToolSpec struct {
	Name        string
	Description string
	Schema      map[string]any
}

// CallResult is the normalized result of an MCP tools/call.
type CallResult struct {
	Text    string
	IsError bool
}

// Client is the MCP transport port (Streamable HTTP in infra/mcp). It discovers
// a server's tools and invokes them; it never knows any specific tool name.
type Client interface {
	// ListTools returns the tools advertised by the server.
	ListTools(ctx context.Context, conn *entity.ServerConnection) ([]ToolSpec, error)
	// CallTool invokes a tool with JSON arguments and returns its result.
	CallTool(ctx context.Context, conn *entity.ServerConnection, tool string, args map[string]any) (CallResult, error)
}

// CreateServer registers an MCP server. The tenant comes from context.
type CreateServer struct {
	Name       string
	BaseURL    string
	AuthHeader string
	AuthToken  string
	Kind       string
}

// UpdateServer carries optional fields; nil pointers mean "leave unchanged".
type UpdateServer struct {
	Name       *string
	BaseURL    *string
	AuthHeader *string
	AuthToken  *string
	Kind       *string
	Enabled    *bool
}

// RunTool is a manual tool execution requested by an agent.
type RunTool struct {
	ConversationID string
	ServerID       string
	Tool           string
	Args           map[string]any
}

// RunResult is the outcome of a manual run. For a write tool, Executed is false
// and Approval carries the pending confirmation card.
type RunResult struct {
	Executed bool             `json:"executed"`
	Result   string           `json:"result,omitempty"`
	Approval *entity.Approval `json:"approval,omitempty"`
	Tool     string           `json:"tool"`
	Write    bool             `json:"write"`
}
