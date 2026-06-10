// Package repository declares the IAM persistence contracts. Implementations
// live in infra/database/mongodb/repositories/iam. Every method is tenant-scoped
// via the context (RequireTenant); the tenant is never taken from the caller's
// arguments.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// RoleRepository persists roles within a tenant.
type RoleRepository interface {
	Create(ctx context.Context, r *entity.Role) error
	Update(ctx context.Context, r *entity.Role) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.Role, error)
	// FindByIDs returns the roles matching the given ids (within the tenant),
	// used to resolve a user's effective permissions.
	FindByIDs(ctx context.Context, ids []string) ([]*entity.Role, error)
	FindByName(ctx context.Context, name string) (*entity.Role, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.Role, error)
}
