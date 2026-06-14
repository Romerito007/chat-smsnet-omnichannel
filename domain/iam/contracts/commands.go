// Package contracts holds the input commands for the IAM services. They are
// transport-agnostic (no HTTP/BSON tags) and validated in the service layer.
package contracts

import "github.com/romerito007/chat-smsnet-omnichannel/domain/authz"

// CreateUser is the input to create a user. The tenant is taken from context.
type CreateUser struct {
	Name               string
	Email              string
	Password           string
	RoleIDs            []string
	SectorIDs          []string
	MaxConcurrentChats int
}

// UpdateUser carries optional fields; nil pointers mean "leave unchanged".
type UpdateUser struct {
	Name               *string
	Password           *string
	Status             *string
	RoleIDs            *[]string
	SectorIDs          *[]string
	MaxConcurrentChats *int
}

// UpdateProfile is the input to PATCH /v1/me: a user editing their own profile.
// Nil pointers mean "leave unchanged".
type UpdateProfile struct {
	Name               *string
	AvatarAttachmentID *string
	// Preferences, when non-nil, FULL-REPLACES the user's stored UI preferences
	// (theme, audio alerts, browser push, …). Free/nested object; the service
	// validates only the enum-constrained fields. Nil = leave unchanged.
	Preferences *map[string]any
}

// CreateRole is the input to create a role.
type CreateRole struct {
	Name        string
	Permissions []authz.Permission
	SectorScope authz.SectorScope
}

// UpdateRole carries optional fields; nil pointers mean "leave unchanged".
type UpdateRole struct {
	Name        *string
	Permissions *[]authz.Permission
	SectorScope *authz.SectorScope
}
