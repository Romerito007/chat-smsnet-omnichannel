// Package mcp holds the request/response DTOs for the MCP server config, the
// conversation-scoped tool endpoints and the approval flow. The auth token is
// never returned (only whether one is set).
package mcp

import (
	"time"

	mcpcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/contracts"
	mcpentity "github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
)

// CreateServerRequest is the body of POST /v1/mcp/servers.
type CreateServerRequest struct {
	Name       string `json:"name"`
	BaseURL    string `json:"base_url"`
	AuthHeader string `json:"auth_header"`
	AuthToken  string `json:"auth_token"`
	Kind       string `json:"kind"`
}

// ToCommand maps to the service command.
func (r CreateServerRequest) ToCommand() mcpcontracts.CreateServer {
	return mcpcontracts.CreateServer{
		Name: r.Name, BaseURL: r.BaseURL, AuthHeader: r.AuthHeader, AuthToken: r.AuthToken, Kind: r.Kind,
	}
}

// UpdateServerRequest is the body of PATCH /v1/mcp/servers/{id}.
type UpdateServerRequest struct {
	Name       *string `json:"name"`
	BaseURL    *string `json:"base_url"`
	AuthHeader *string `json:"auth_header"`
	AuthToken  *string `json:"auth_token"`
	Kind       *string `json:"kind"`
	Enabled    *bool   `json:"enabled"`
}

// ToCommand maps to the service command.
func (r UpdateServerRequest) ToCommand() mcpcontracts.UpdateServer {
	return mcpcontracts.UpdateServer{
		Name: r.Name, BaseURL: r.BaseURL, AuthHeader: r.AuthHeader,
		AuthToken: r.AuthToken, Kind: r.Kind, Enabled: r.Enabled,
	}
}

// ServerResponse is the public representation of an MCP server. The auth token is
// never returned — only whether one is set.
type ServerResponse struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Name       string    `json:"name"`
	Transport  string    `json:"transport"`
	BaseURL    string    `json:"base_url"`
	AuthHeader string    `json:"auth_header,omitempty"`
	HasAuth    bool      `json:"has_auth"`
	Kind       string    `json:"kind"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// NewServerResponse maps a server entity, masking the auth token.
func NewServerResponse(s *mcpentity.ServerConnection) ServerResponse {
	return ServerResponse{
		ID: s.ID, TenantID: s.TenantID, Name: s.Name, Transport: string(s.Transport),
		BaseURL: s.BaseURL, AuthHeader: s.AuthHeader, HasAuth: s.AuthToken != "",
		Kind: string(s.Kind), Enabled: s.Enabled, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

// NewServerResponses maps a slice.
func NewServerResponses(items []*mcpentity.ServerConnection) []ServerResponse {
	out := make([]ServerResponse, len(items))
	for i, s := range items {
		out[i] = NewServerResponse(s)
	}
	return out
}

// ToolResponse is a discovered tool annotated read/write.
type ToolResponse struct {
	ServerID    string         `json:"server_id"`
	ServerName  string         `json:"server_name"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
	Write       bool           `json:"write"`
}

// NewToolResponses maps discovered tools.
func NewToolResponses(tools []mcpentity.Tool) []ToolResponse {
	out := make([]ToolResponse, len(tools))
	for i, t := range tools {
		out[i] = ToolResponse{
			ServerID: t.ServerID, ServerName: t.ServerName, Name: t.Name,
			Description: t.Description, Schema: t.Schema, Write: t.Write,
		}
	}
	return out
}

// RunToolRequest is the body of POST /v1/conversations/{id}/mcp/run.
type RunToolRequest struct {
	ServerID string         `json:"server_id"`
	Tool     string         `json:"tool"`
	Args     map[string]any `json:"args"`
}

// DecideRequest is the body of POST /v1/conversations/{id}/copilot/approvals/{id}.
type DecideRequest struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason"`
}

// ApprovalResponse is the read view of a write-action approval.
type ApprovalResponse struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversation_id"`
	ServerName     string         `json:"server_name,omitempty"`
	Tool           string         `json:"tool"`
	Args           map[string]any `json:"args,omitempty"`
	Status         string         `json:"status"`
	ProposedBy     string         `json:"proposed_by,omitempty"`
	DecidedBy      string         `json:"decided_by,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	DecidedAt      *time.Time     `json:"decided_at,omitempty"`
}

// NewApprovalResponses maps a slice of approvals (empty slice when none).
func NewApprovalResponses(items []*mcpentity.Approval) []ApprovalResponse {
	out := make([]ApprovalResponse, 0, len(items))
	for _, a := range items {
		out = append(out, ApprovalResponse{
			ID: a.ID, ConversationID: a.ConversationID, ServerName: a.ServerName, Tool: a.Tool,
			Args: a.Args, Status: string(a.Status), ProposedBy: a.ProposedBy, DecidedBy: a.DecidedBy,
			Reason: a.Reason, CreatedAt: a.CreatedAt, DecidedAt: a.DecidedAt,
		})
	}
	return out
}

// CallLogResponse is the read view of a payload-free tool-call log.
type CallLogResponse struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	ServerName     string    `json:"server_name,omitempty"`
	Tool           string    `json:"tool"`
	Write          bool      `json:"write"`
	Status         string    `json:"status"`
	LatencyMs      int64     `json:"latency_ms"`
	ErrorSummary   string    `json:"error_summary,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// NewCallLogResponses maps a slice of call logs (empty slice when none).
func NewCallLogResponses(items []*mcpentity.CallLog) []CallLogResponse {
	out := make([]CallLogResponse, 0, len(items))
	for _, l := range items {
		out = append(out, CallLogResponse{
			ID: l.ID, ConversationID: l.ConversationID, ServerName: l.ServerName, Tool: l.Tool,
			Write: l.Write, Status: string(l.Status), LatencyMs: l.LatencyMs,
			ErrorSummary: l.ErrorSummary, CreatedAt: l.CreatedAt,
		})
	}
	return out
}
