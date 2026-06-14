// Package repository declares the Tenant persistence contract. The Mongo
// implementation lives in infra/database/mongodb/repositories/tenant.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/entity"
)

// TenantRepository persists tenants. Tenants are the isolation root, so lookups
// are by id (not tenant-scoped like other repos).
type TenantRepository interface {
	// Create inserts a new tenant (used by self-service signup).
	Create(ctx context.Context, t *entity.Tenant) error
	// FindByID returns the tenant or a not_found AppError.
	FindByID(ctx context.Context, id string) (*entity.Tenant, error)
	// FindByExternalRef returns the tenant with the given provisioner external_ref,
	// or a not_found AppError. Used for durable provisioning idempotency.
	FindByExternalRef(ctx context.Context, ref string) (*entity.Tenant, error)
	// Update persists mutable fields (name, status, settings).
	Update(ctx context.Context, t *entity.Tenant) error
	// ListActive returns every active tenant. Used by periodic jobs to fan work
	// out across tenants.
	ListActive(ctx context.Context) ([]*entity.Tenant, error)
}
