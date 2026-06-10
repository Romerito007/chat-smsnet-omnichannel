// Package authz holds the permission vocabulary and the authorization decision
// contract. Concrete role/permission storage lives in its own domain; this
// package only defines the primitives the rest of the system checks against.
package authz

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Permission is a fine-grained capability, e.g. "conversation:read".
type Permission string

// Role groups a set of permissions. These are the seeded defaults referenced by
// bootstrap_seeds.
const (
	RoleOwner Role = "owner"
	RoleAdmin Role = "admin"
	RoleAgent Role = "agent"
)

// Role is a named bundle of permissions.
type Role string

// Authorizer decides whether an actor may perform an action. The default
// implementation is permissive enough to compile and run; real policy is layered
// on top via domain/policy and the authz domain repository.
type Authorizer interface {
	// Authorize returns nil when the actor holds the permission, or a forbidden
	// AppError otherwise.
	Authorize(ctx context.Context, actor shared.Actor, perm Permission) error
}
