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
