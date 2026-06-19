// Package iam holds the request/response DTOs for the IAM endpoints. DTOs never
// expose secrets (e.g. password_hash) and decouple the wire shape from entities.
package iam

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
)

// ── Users ────────────────────────────────────────────────────────────────────

// CreateUserRequest is the body of POST /v1/users.
type CreateUserRequest struct {
	Name               string   `json:"name"`
	Email              string   `json:"email"`
	Password           string   `json:"password"`
	RoleIDs            []string `json:"role_ids"`
	SectorIDs          []string `json:"sector_ids"`
	MaxConcurrentChats int      `json:"max_concurrent_chats"`
}

// ToCommand maps the request to the service command.
func (r CreateUserRequest) ToCommand() contracts.CreateUser {
	return contracts.CreateUser{
		Name:               r.Name,
		Email:              r.Email,
		Password:           r.Password,
		RoleIDs:            r.RoleIDs,
		SectorIDs:          r.SectorIDs,
		MaxConcurrentChats: r.MaxConcurrentChats,
	}
}

// UpdateUserRequest is the body of PATCH /v1/users/{id}. Nil fields are left
// unchanged.
type UpdateUserRequest struct {
	Name               *string   `json:"name"`
	Password           *string   `json:"password"`
	Status             *string   `json:"status"`
	RoleIDs            *[]string `json:"role_ids"`
	SectorIDs          *[]string `json:"sector_ids"`
	MaxConcurrentChats *int      `json:"max_concurrent_chats"`
}

// ToCommand maps the request to the service command.
func (r UpdateUserRequest) ToCommand() contracts.UpdateUser {
	return contracts.UpdateUser{
		Name:               r.Name,
		Password:           r.Password,
		Status:             r.Status,
		RoleIDs:            r.RoleIDs,
		SectorIDs:          r.SectorIDs,
		MaxConcurrentChats: r.MaxConcurrentChats,
	}
}

// UserResponse is the public representation of a user (no password).
type UserResponse struct {
	ID                 string   `json:"id"`
	TenantID           string   `json:"tenant_id"`
	Name               string   `json:"name"`
	Email              string   `json:"email"`
	Status             string   `json:"status"`
	RoleIDs            []string `json:"role_ids"`
	SectorIDs          []string `json:"sector_ids"`
	MaxConcurrentChats int      `json:"max_concurrent_chats"`
	AvatarAttachmentID string   `json:"avatar_attachment_id,omitempty"`
	// PresenceAvailability is the agent's durable manual availability
	// (online|away|offline; defaults to online); AutoOffline toggles going offline
	// when the last socket drops while online (defaults to true). Read-only here —
	// changed via the presence endpoints.
	PresenceAvailability string `json:"presence_availability"`
	AutoOffline          bool   `json:"auto_offline"`
	// AvatarURL is a short-lived signed URL the browser loads directly (no JWT).
	// Read-only/derived; present only when the avatar exists and is ready.
	AvatarURL string `json:"avatar_url,omitempty"`
	// Preferences is the user's UI preferences umbrella (theme, audio_alerts,
	// browser_push, …), echoed back as stored. Omitted when the user has none yet
	// (the front then applies its defaults).
	Preferences map[string]any `json:"preferences,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// NewUserResponse maps a user entity to its DTO.
func NewUserResponse(u *entity.User) UserResponse {
	return UserResponse{
		ID:                   u.ID,
		TenantID:             u.TenantID,
		Name:                 u.Name,
		Email:                u.Email,
		Status:               string(u.Status),
		RoleIDs:              u.RoleIDs,
		SectorIDs:            u.SectorIDs,
		MaxConcurrentChats:   u.MaxConcurrentChats,
		AvatarAttachmentID:   u.AvatarAttachmentID,
		PresenceAvailability: u.AvailabilityOr(),
		AutoOffline:          u.AutoOfflineOr(),
		Preferences:          u.Preferences,
		CreatedAt:            u.CreatedAt,
		UpdatedAt:            u.UpdatedAt,
	}
}

// NewUserResponses maps a slice of users.
func NewUserResponses(users []*entity.User) []UserResponse {
	return NewUserResponsesWithAvatars(users, nil)
}

// NewUserResponsesWithAvatars maps a slice, attaching each user's signed avatar
// URL from avatarURLs (keyed by avatar attachment id; resolved in batch).
func NewUserResponsesWithAvatars(users []*entity.User, avatarURLs map[string]string) []UserResponse {
	out := make([]UserResponse, len(users))
	for i, u := range users {
		r := NewUserResponse(u)
		if u.AvatarAttachmentID != "" {
			r.AvatarURL = avatarURLs[u.AvatarAttachmentID]
		}
		out[i] = r
	}
	return out
}

// NewUserResponseWithAvatar maps one user, attaching its signed avatar URL.
func NewUserResponseWithAvatar(u *entity.User, avatarURLs map[string]string) UserResponse {
	r := NewUserResponse(u)
	if u.AvatarAttachmentID != "" {
		r.AvatarURL = avatarURLs[u.AvatarAttachmentID]
	}
	return r
}

// ── Roles ────────────────────────────────────────────────────────────────────

// CreateRoleRequest is the body of POST /v1/roles.
type CreateRoleRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	SectorScope string   `json:"sector_scope"`
}

// ToCommand maps the request to the service command.
func (r CreateRoleRequest) ToCommand() contracts.CreateRole {
	return contracts.CreateRole{
		Name:        r.Name,
		Permissions: toPerms(r.Permissions),
		SectorScope: authz.SectorScope(r.SectorScope),
	}
}

// UpdateRoleRequest is the body of PATCH /v1/roles/{id}.
type UpdateRoleRequest struct {
	Name        *string   `json:"name"`
	Permissions *[]string `json:"permissions"`
	SectorScope *string   `json:"sector_scope"`
}

// ToCommand maps the request to the service command.
func (r UpdateRoleRequest) ToCommand() contracts.UpdateRole {
	cmd := contracts.UpdateRole{Name: r.Name}
	if r.Permissions != nil {
		p := toPerms(*r.Permissions)
		cmd.Permissions = &p
	}
	if r.SectorScope != nil {
		s := authz.SectorScope(*r.SectorScope)
		cmd.SectorScope = &s
	}
	return cmd
}

// RoleResponse is the public representation of a role.
type RoleResponse struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	Permissions []string  `json:"permissions"`
	SectorScope string    `json:"sector_scope"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewRoleResponse maps a role entity to its DTO.
func NewRoleResponse(r *entity.Role) RoleResponse {
	return RoleResponse{
		ID:          r.ID,
		TenantID:    r.TenantID,
		Name:        r.Name,
		Permissions: fromPerms(r.Permissions),
		SectorScope: string(r.SectorScope),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// NewRoleResponses maps a slice of roles.
func NewRoleResponses(roles []*entity.Role) []RoleResponse {
	out := make([]RoleResponse, len(roles))
	for i, r := range roles {
		out[i] = NewRoleResponse(r)
	}
	return out
}

func toPerms(ss []string) []authz.Permission {
	out := make([]authz.Permission, 0, len(ss))
	for _, s := range ss {
		out = append(out, authz.Permission(s))
	}
	return out
}

func fromPerms(perms []authz.Permission) []string {
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = string(p)
	}
	return out
}
