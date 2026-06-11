// Package repository declares the audit-log persistence contract. The
// implementation lives in infra/database/mongodb/repositories/audit and is
// tenant-scoped via the context.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/audit/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Repository persists and queries audit logs for a tenant.
type Repository interface {
	// Create appends an audit log.
	Create(ctx context.Context, l *entity.AuditLog) error
	// List returns audit logs newest-first (keyset pagination), optionally
	// filtered by action prefix and resource id when set on the filter.
	List(ctx context.Context, f Filter, page shared.PageRequest) ([]*entity.AuditLog, error)
}

// Filter narrows an audit-log listing. Empty fields are ignored.
type Filter struct {
	Action     string
	ResourceID string
}
