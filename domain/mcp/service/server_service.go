// Package service holds the MCP business logic: server registration, dynamic
// tool discovery, the per-tenant tool registry, manual tool execution and the
// human-approval flow for write actions.
package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ServerService manages per-tenant MCP server registrations and discovers their
// tools dynamically (never hard-coding any tool name).
type ServerService struct {
	repo    repository.ServerRepository
	client  contracts.Client
	clock   shared.Clock
	auditor shared.Auditor
}

// NewServerService builds the service.
func NewServerService(repo repository.ServerRepository, client contracts.Client, clock shared.Clock) *ServerService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &ServerService{repo: repo, client: client, clock: clock, auditor: shared.NoopAuditor{}}
}

// SetAuditor wires the audit trail. Optional.
func (s *ServerService) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// Create registers an MCP server for the current tenant.
func (s *ServerService) Create(ctx context.Context, cmd contracts.CreateServer) (*entity.ServerConnection, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(cmd.Name)
	baseURL := strings.TrimSpace(cmd.BaseURL)
	kind := entity.Kind(strings.TrimSpace(cmd.Kind))
	v := map[string]any{}
	if name == "" {
		v["name"] = "is required"
	}
	if baseURL == "" {
		v["base_url"] = "is required"
	}
	if !kind.Valid() {
		v["kind"] = "must be read or write"
	}
	if len(v) > 0 {
		return nil, apperror.Validation("validation failed").WithDetails(v)
	}
	now := s.clock.Now()
	conn := &entity.ServerConnection{
		ID:         shared.NewID(),
		TenantID:   tenantID,
		Name:       name,
		Transport:  entity.TransportStreamableHTTP,
		BaseURL:    baseURL,
		AuthHeader: strings.TrimSpace(cmd.AuthHeader),
		AuthToken:  cmd.AuthToken,
		Kind:       kind,
		Enabled:    true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repo.Create(ctx, conn); err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "mcp.server.created", ResourceType: "mcp_server", ResourceID: conn.ID,
		Data: map[string]any{"name": conn.Name, "kind": string(conn.Kind)},
	})
	return conn, nil
}

// Update applies the non-nil fields of cmd.
func (s *ServerService) Update(ctx context.Context, id string, cmd contracts.UpdateServer) (*entity.ServerConnection, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	conn, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		conn.Name = strings.TrimSpace(*cmd.Name)
	}
	if cmd.BaseURL != nil {
		conn.BaseURL = strings.TrimSpace(*cmd.BaseURL)
	}
	if cmd.AuthHeader != nil {
		conn.AuthHeader = strings.TrimSpace(*cmd.AuthHeader)
	}
	if cmd.AuthToken != nil {
		conn.AuthToken = *cmd.AuthToken
	}
	if cmd.Kind != nil {
		k := entity.Kind(strings.TrimSpace(*cmd.Kind))
		if !k.Valid() {
			return nil, apperror.Validation("kind must be read or write")
		}
		conn.Kind = k
	}
	if cmd.Enabled != nil {
		conn.Enabled = *cmd.Enabled
	}
	conn.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, conn); err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "mcp.server.updated", ResourceType: "mcp_server", ResourceID: conn.ID,
	})
	return conn, nil
}

// Delete removes a server.
func (s *ServerService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "mcp.server.deleted", ResourceType: "mcp_server", ResourceID: id,
	})
	return nil
}

// Get returns a server by id.
func (s *ServerService) Get(ctx context.Context, id string) (*entity.ServerConnection, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns a page of servers.
func (s *ServerService) List(ctx context.Context, page shared.PageRequest) ([]*entity.ServerConnection, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, page.Normalize())
}

// Test lists the server's tools dynamically, verifying connectivity + discovery.
func (s *ServerService) Test(ctx context.Context, id string) ([]entity.Tool, error) {
	conn, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	specs, err := s.client.ListTools(ctx, conn)
	if err != nil {
		return nil, apperror.Integration("could not list tools from the MCP server").Wrap(err)
	}
	return annotate(conn, specs), nil
}

// annotate maps discovered specs to domain tools, marking write by server kind.
func annotate(conn *entity.ServerConnection, specs []contracts.ToolSpec) []entity.Tool {
	out := make([]entity.Tool, 0, len(specs))
	for _, sp := range specs {
		out = append(out, entity.Tool{
			ServerID: conn.ID, ServerName: conn.Name, Name: sp.Name,
			Description: sp.Description, Schema: sp.Schema, Write: conn.Kind == entity.KindWrite,
		})
	}
	return out
}
