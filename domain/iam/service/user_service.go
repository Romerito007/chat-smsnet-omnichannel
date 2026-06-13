package service

import (
	"context"
	"net/mail"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// minPasswordLen is the minimum acceptable password length.
const minPasswordLen = 8

// UserService manages tenant users.
type UserService struct {
	users      repository.UserRepository
	hasher     iam.PasswordHasher
	clock      shared.Clock
	auditor    shared.Auditor
	avatarURLs shared.AvatarURLResolver
}

// NewUserService builds the service.
func NewUserService(users repository.UserRepository, hasher iam.PasswordHasher, clock shared.Clock) *UserService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &UserService{users: users, hasher: hasher, clock: clock, auditor: shared.NoopAuditor{}}
}

// SetAuditor wires the audit trail. Optional: when unset, user changes are not
// audited.
func (s *UserService) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// SetAvatarURLResolver wires the resolver that turns avatar_attachment_ids into
// short-lived signed avatar URLs for user payloads. Optional.
func (s *UserService) SetAvatarURLResolver(r shared.AvatarURLResolver) {
	if r != nil {
		s.avatarURLs = r
	}
}

// AvatarURLs batch-resolves a set of avatar attachment ids to signed URLs, keyed
// by attachment id. Best-effort and nil-safe (returns nil when unwired).
func (s *UserService) AvatarURLs(ctx context.Context, attachmentIDs []string) (map[string]string, error) {
	if s.avatarURLs == nil || len(attachmentIDs) == 0 {
		return nil, nil
	}
	return s.avatarURLs.SignedAvatarURLs(ctx, attachmentIDs)
}

// Create validates and persists a new user within the current tenant, hashing
// the password. Email uniqueness is enforced by the repository (unique index →
// conflict).
func (s *UserService) Create(ctx context.Context, cmd contracts.CreateUser) (*entity.User, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}

	v := map[string]any{}
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		v["name"] = "is required"
	}
	email := normalizeEmail(cmd.Email)
	if !validEmail(email) {
		v["email"] = "must be a valid email"
	}
	if len(cmd.Password) < minPasswordLen {
		v["password"] = "must be at least 8 characters"
	}
	if len(v) > 0 {
		return nil, apperror.Validation("validation failed").WithDetails(v)
	}

	hash, err := s.hasher.Hash(cmd.Password)
	if err != nil {
		return nil, apperror.Internal("could not hash password").Wrap(err)
	}

	now := s.clock.Now()
	maxChats := cmd.MaxConcurrentChats
	if maxChats <= 0 {
		maxChats = 1
	}
	user := &entity.User{
		ID:                 shared.NewID(),
		TenantID:           tenantID,
		Name:               name,
		Email:              email,
		PasswordHash:       hash,
		Status:             entity.StatusActive,
		RoleIDs:            cmd.RoleIDs,
		SectorIDs:          entity.NormalizeSectorIDs(cmd.SectorIDs),
		MaxConcurrentChats: maxChats,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "user.created", ResourceType: "user", ResourceID: user.ID,
		Data: map[string]any{"email": user.Email, "role_ids": user.RoleIDs},
	})
	return user, nil
}

// Update applies the non-nil fields of cmd to the user.
func (s *UserService) Update(ctx context.Context, id string, cmd contracts.UpdateUser) (*entity.User, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if cmd.Name != nil {
		name := strings.TrimSpace(*cmd.Name)
		if name == "" {
			return nil, apperror.Validation("name cannot be empty")
		}
		user.Name = name
	}
	if cmd.Password != nil {
		if len(*cmd.Password) < minPasswordLen {
			return nil, apperror.Validation("password must be at least 8 characters")
		}
		hash, err := s.hasher.Hash(*cmd.Password)
		if err != nil {
			return nil, apperror.Internal("could not hash password").Wrap(err)
		}
		user.PasswordHash = hash
	}
	if cmd.Status != nil {
		status := entity.Status(*cmd.Status)
		if status != entity.StatusActive && status != entity.StatusDisabled {
			return nil, apperror.Validation("invalid status")
		}
		user.Status = status
	}
	if cmd.RoleIDs != nil {
		user.RoleIDs = *cmd.RoleIDs
	}
	if cmd.SectorIDs != nil {
		user.SectorIDs = entity.NormalizeSectorIDs(*cmd.SectorIDs)
	}
	if cmd.MaxConcurrentChats != nil {
		if *cmd.MaxConcurrentChats < 0 {
			return nil, apperror.Validation("max_concurrent_chats cannot be negative")
		}
		user.MaxConcurrentChats = *cmd.MaxConcurrentChats
	}
	user.UpdatedAt = s.clock.Now()
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	data := map[string]any{}
	if cmd.RoleIDs != nil {
		// A role re-assignment is a permission change ("alteração de permissões").
		data["permissions_changed"] = true
		data["role_ids"] = user.RoleIDs
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "user.updated", ResourceType: "user", ResourceID: user.ID, Data: data,
	})
	return user, nil
}

// UpdateProfile lets a user edit their own profile (name and avatar). The id is
// the authenticated user's own id, resolved by the controller from the token.
func (s *UserService) UpdateProfile(ctx context.Context, id string, cmd contracts.UpdateProfile) (*entity.User, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		name := strings.TrimSpace(*cmd.Name)
		if name == "" {
			return nil, apperror.Validation("name cannot be empty")
		}
		user.Name = name
	}
	if cmd.AvatarAttachmentID != nil {
		user.AvatarAttachmentID = strings.TrimSpace(*cmd.AvatarAttachmentID)
	}
	user.UpdatedAt = s.clock.Now()
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// ChangePassword verifies the user's current password and sets a new one. The id
// is the authenticated user's own id.
func (s *UserService) ChangePassword(ctx context.Context, id, current, next string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if len(next) < minPasswordLen {
		return apperror.Validation("new_password must be at least 8 characters")
	}
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.hasher.Compare(user.PasswordHash, current); err != nil {
		return apperror.Unauthorized("current password is incorrect")
	}
	hash, err := s.hasher.Hash(next)
	if err != nil {
		return apperror.Internal("could not hash password").Wrap(err)
	}
	user.PasswordHash = hash
	user.UpdatedAt = s.clock.Now()
	if err := s.users.Update(ctx, user); err != nil {
		return err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "auth.password_changed", ResourceType: "user", ResourceID: user.ID,
	})
	return nil
}

// Delete removes a user.
func (s *UserService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if err := s.users.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "user.deleted", ResourceType: "user", ResourceID: id,
	})
	return nil
}

// Get returns a user by id.
func (s *UserService) Get(ctx context.Context, id string) (*entity.User, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.users.FindByID(ctx, id)
}

// List returns a page of users for the tenant.
func (s *UserService) List(ctx context.Context, page shared.PageRequest) ([]*entity.User, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.users.List(ctx, page.Normalize())
}

// ListBySector returns the active users that belong to the sector — the exact set
// the routing assign accepts for it (same source of truth: the user's sector_ids).
// Used to filter the assignable-agents directory so the front never receives an
// agent the assign would reject for membership.
func (s *UserService) ListBySector(ctx context.Context, sectorID string) ([]*entity.User, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.users.ListBySector(ctx, sectorID)
}

// ResolveEffective computes a user's effective permission set and sector scope
// from its roles. Scope is ScopeAll when any role grants it; otherwise ScopeOwn.
func ResolveEffective(roles []*entity.Role) ([]authz.Permission, authz.SectorScope) {
	set := map[authz.Permission]struct{}{}
	scope := authz.ScopeOwn
	for _, r := range roles {
		if r.SectorScope == authz.ScopeAll {
			scope = authz.ScopeAll
		}
		for _, p := range r.Permissions {
			set[p] = struct{}{}
		}
	}
	perms := make([]authz.Permission, 0, len(set))
	for _, p := range authz.AllPermissions() {
		if _, ok := set[p]; ok {
			perms = append(perms, p)
		}
	}
	return perms, scope
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	if email == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}
