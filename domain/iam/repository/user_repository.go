package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// UserRepository persists users within a tenant.
type UserRepository interface {
	Create(ctx context.Context, u *entity.User) error
	Update(ctx context.Context, u *entity.User) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.User, error)
	// FindByIDs batch-loads users by id within the tenant (missing ids absent),
	// used to resolve agent display cards for the inbox without N+1.
	FindByIDs(ctx context.Context, ids []string) ([]*entity.User, error)
	// FindByEmail looks a user up by email within the current tenant scope.
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
	// FindByEmailAnyTenant looks a user up by email across all tenants. It is
	// used only by login, which is pre-authentication and has no tenant scope
	// yet; the matched record carries the authoritative tenant.
	FindByEmailAnyTenant(ctx context.Context, email string) (*entity.User, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.User, error)
	// ListBySector returns the active users who belong to the given sector
	// (within the tenant). Used by routing to find eligible agents.
	ListBySector(ctx context.Context, sectorID string) ([]*entity.User, error)
	// SetPresenceSettings persists the agent's durable presence availability and/or
	// auto-offline toggle with a focused $set (nil fields unchanged), so it never
	// clobbers a concurrent profile edit.
	SetPresenceSettings(ctx context.Context, userID string, availability *string, autoOffline *bool) error
}
