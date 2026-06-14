package contracts

import "context"

// ISPToolBridge lets the host gate and decorate the SMSNET MCP tool calls without
// the model ever seeing ISP credentials. It is the single seam where the ISP
// config{type+creds} enters a tool call. Implemented by the copilot side (which
// resolves the conversation's assistant → ISP profile); only SMSNET servers are
// passed here.
type ISPToolBridge interface {
	// ToolSource resolves the conversation channel's EXTERNAL tool source from its
	// assistant: ISP (SMSNET), a specific MCP server, or none. It is the single
	// decision point for which servers the copilot session may see.
	ToolSource(ctx context.Context, channelID string) (ToolSource, error)
	// AllowServer reports whether a SMSNET server is exposed to the AI for a
	// conversation's channel (by connection id), based on the assistant's pinned
	// ISP profile: no assistant/profile → false (no SMSNET tools); read server →
	// true; write server → only when the profile supports liberacao or chamado.
	AllowServer(ctx context.Context, channelID, serverName string, write bool) (bool, error)
	// Decorate injects the ISP config (and, for writes, an idempotency key) into a
	// SMSNET tool call's arguments, returning the args to send. It OVERWRITES any
	// client-supplied "config" so the model can never inject its own credentials.
	Decorate(ctx context.Context, in DecorateInput) (map[string]any, error)
}

// ToolSourceKind is the external tool source an assistant selects.
type ToolSourceKind string

const (
	ToolSourceNone ToolSourceKind = "none" // no assistant/source → no external tools
	ToolSourceISP  ToolSourceKind = "isp"  // SMSNET tools via the pinned ISP profile
	ToolSourceMCP  ToolSourceKind = "mcp"  // a specific tenant MCP server's tools
)

// ToolSource is the resolved external tool source for a conversation's channel.
type ToolSource struct {
	Kind        ToolSourceKind
	MCPServerID string // set only when Kind == ToolSourceMCP
}

// DecorateInput is the context for one SMSNET tool-call decoration.
type DecorateInput struct {
	ChannelID      string
	ServerName     string
	Write          bool
	IdempotencyKey string // for write calls (e.g. the approval id)
	Args           map[string]any
}
