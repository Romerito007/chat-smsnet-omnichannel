// Package service holds the IAM business logic (users and roles). All reads and
// writes are tenant-scoped via the context.
package service

import (
	"context"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// RoleService manages tenant roles.
type RoleService struct {
	roles   repository.RoleRepository
	clock   shared.Clock
	auditor shared.Auditor
}

// NewRoleService builds the service.
func NewRoleService(roles repository.RoleRepository, clock shared.Clock) *RoleService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &RoleService{roles: roles, clock: clock, auditor: shared.NoopAuditor{}}
}

// SetAuditor wires the audit trail. Optional: when unset, role/permission changes
// are not audited.
func (s *RoleService) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// Create validates and persists a new role within the current tenant.
func (s *RoleService) Create(ctx context.Context, cmd contracts.CreateRole) (*entity.Role, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, apperror.Validation("role name is required").
			WithDetails(map[string]any{"name": "is required"})
	}
	scope := cmd.SectorScope
	if !scope.Valid() {
		scope = authz.ScopeOwn
	}

	now := s.clock.Now()
	role := &entity.Role{
		ID:          shared.NewID(),
		TenantID:    tenantID,
		Name:        name,
		Permissions: authz.SanitizePermissions(cmd.Permissions),
		SectorScope: scope,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.roles.Create(ctx, role); err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "role.created", ResourceType: "role", ResourceID: role.ID,
		Data: map[string]any{"name": role.Name, "permissions": role.Permissions},
	})
	return role, nil
}

// SeedDefaults idempotently creates the default roles (owner/admin/agent) for a
// tenant and returns a name→id map. Used by self-service signup to provision a
// brand-new tenant. The context must already be scoped to the new tenant.
func (s *RoleService) SeedDefaults(ctx context.Context) (map[string]string, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, def := range authz.DefaultRoles() {
		role, err := s.Create(ctx, contracts.CreateRole{
			Name:        def.Name,
			Permissions: def.Permissions,
			SectorScope: def.SectorScope,
		})
		if err != nil {
			// Tolerate an already-seeded role (idempotent re-provisioning).
			if apperror.From(err).Code == apperror.CodeConflict {
				if existing, ferr := s.roles.FindByName(ctx, def.Name); ferr == nil {
					out[def.Name] = existing.ID
					continue
				}
			}
			return nil, err
		}
		out[def.Name] = role.ID
	}
	return out, nil
}

// Update applies the non-nil fields of cmd to the role.
func (s *RoleService) Update(ctx context.Context, id string, cmd contracts.UpdateRole) (*entity.Role, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	role, err := s.roles.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		name := strings.TrimSpace(*cmd.Name)
		if name == "" {
			return nil, apperror.Validation("role name cannot be empty")
		}
		role.Name = name
	}
	if cmd.Permissions != nil {
		role.Permissions = authz.SanitizePermissions(*cmd.Permissions)
	}
	if cmd.SectorScope != nil {
		if !cmd.SectorScope.Valid() {
			return nil, apperror.Validation("invalid sector scope")
		}
		role.SectorScope = *cmd.SectorScope
	}
	role.UpdatedAt = s.clock.Now()
	if err := s.roles.Update(ctx, role); err != nil {
		return nil, err
	}
	data := map[string]any{"name": role.Name}
	if cmd.Permissions != nil {
		// Editing a role's permission set is a permission change.
		data["permissions_changed"] = true
		data["permissions"] = role.Permissions
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "role.updated", ResourceType: "role", ResourceID: role.ID, Data: data,
	})
	return role, nil
}

// Delete removes a role.
func (s *RoleService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	if err := s.roles.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "role.deleted", ResourceType: "role", ResourceID: id,
	})
	return nil
}

// Get returns a role by id.
func (s *RoleService) Get(ctx context.Context, id string) (*entity.Role, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.roles.FindByID(ctx, id)
}

// List returns a page of roles for the tenant.
func (s *RoleService) List(ctx context.Context, page shared.PageRequest) ([]*entity.Role, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.roles.List(ctx, page.Normalize())
}
