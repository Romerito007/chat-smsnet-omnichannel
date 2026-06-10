// Package entity holds the IAM aggregates: Role and User.
package entity

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
)

// Role is a named, tenant-scoped bundle of permissions with a sector scope.
type Role struct {
	ID          string
	TenantID    string
	Name        string
	Permissions []authz.Permission
	SectorScope authz.SectorScope
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// HasPermission reports whether the role grants p.
func (r *Role) HasPermission(p authz.Permission) bool {
	for _, rp := range r.Permissions {
		if rp == p {
			return true
		}
	}
	return false
}
