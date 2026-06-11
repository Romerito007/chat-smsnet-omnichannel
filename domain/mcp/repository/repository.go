// Package repository declares the MCP persistence contracts. Implementations live
// in infra/database/mongodb/repositories/mcp; every method is tenant-scoped via
// the context.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ServerRepository persists MCP server connections.
type ServerRepository interface {
	Create(ctx context.Context, s *entity.ServerConnection) error
	Update(ctx context.Context, s *entity.ServerConnection) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.ServerConnection, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.ServerConnection, error)
	// ListEnabled returns every enabled server for the tenant (tool aggregation).
	ListEnabled(ctx context.Context) ([]*entity.ServerConnection, error)
}

// ApprovalRepository persists write-action approvals.
type ApprovalRepository interface {
	Create(ctx context.Context, a *entity.Approval) error
	Update(ctx context.Context, a *entity.Approval) error
	FindByID(ctx context.Context, id string) (*entity.Approval, error)
}

// CallLogRepository persists payload-free tool-call logs.
type CallLogRepository interface {
	Create(ctx context.Context, l *entity.CallLog) error
}
