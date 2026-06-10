package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fakeRoleRepo struct {
	roles map[string]*entity.Role
}

func newFakeRoleRepo() *fakeRoleRepo { return &fakeRoleRepo{roles: map[string]*entity.Role{}} }

func (r *fakeRoleRepo) Create(ctx context.Context, role *entity.Role) error {
	cp := *role
	r.roles[role.ID] = &cp
	return nil
}
func (r *fakeRoleRepo) Update(ctx context.Context, role *entity.Role) error {
	if _, ok := r.roles[role.ID]; !ok {
		return apperror.NotFound("resource not found")
	}
	cp := *role
	r.roles[role.ID] = &cp
	return nil
}
func (r *fakeRoleRepo) Delete(ctx context.Context, id string) error {
	delete(r.roles, id)
	return nil
}
func (r *fakeRoleRepo) FindByID(ctx context.Context, id string) (*entity.Role, error) {
	tenant, _ := shared.TenantFrom(ctx)
	if role, ok := r.roles[id]; ok && role.TenantID == tenant {
		cp := *role
		return &cp, nil
	}
	return nil, apperror.NotFound("resource not found")
}
func (r *fakeRoleRepo) FindByIDs(ctx context.Context, ids []string) ([]*entity.Role, error) {
	tenant, _ := shared.TenantFrom(ctx)
	var out []*entity.Role
	for _, id := range ids {
		if role, ok := r.roles[id]; ok && role.TenantID == tenant {
			cp := *role
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (r *fakeRoleRepo) FindByName(ctx context.Context, name string) (*entity.Role, error) {
	tenant, _ := shared.TenantFrom(ctx)
	for _, role := range r.roles {
		if role.TenantID == tenant && role.Name == name {
			cp := *role
			return &cp, nil
		}
	}
	return nil, apperror.NotFound("resource not found")
}
func (r *fakeRoleRepo) List(ctx context.Context, page shared.PageRequest) ([]*entity.Role, error) {
	tenant, _ := shared.TenantFrom(ctx)
	var out []*entity.Role
	for _, role := range r.roles {
		if role.TenantID == tenant {
			cp := *role
			out = append(out, &cp)
		}
	}
	return out, nil
}

func newRoleService() (*RoleService, *fakeRoleRepo) {
	repo := newFakeRoleRepo()
	return NewRoleService(repo, fixedClock{t: time.Unix(1700000000, 0).UTC()}), repo
}

func TestCreateRole_SanitizesPermissionsAndDefaultsScope(t *testing.T) {
	svc, _ := newRoleService()
	ctx := tenantCtx("t1")

	role, err := svc.Create(ctx, contracts.CreateRole{
		Name:        "Support",
		Permissions: []authz.Permission{authz.ConversationRead, "bogus.permission", authz.MessageSend},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if role.SectorScope != authz.ScopeOwn {
		t.Errorf("default scope = %q, want own", role.SectorScope)
	}
	for _, p := range role.Permissions {
		if !authz.IsValid(p) {
			t.Errorf("invalid permission leaked: %q", p)
		}
	}
	if len(role.Permissions) != 2 {
		t.Errorf("expected 2 valid permissions, got %d", len(role.Permissions))
	}
}

func TestCreateRole_RequiresName(t *testing.T) {
	svc, _ := newRoleService()
	if _, err := svc.Create(tenantCtx("t1"), contracts.CreateRole{Name: "  "}); !isValidation(err) {
		t.Errorf("expected validation_error, got %v", err)
	}
}

func TestRole_TenantIsolation(t *testing.T) {
	svc, _ := newRoleService()
	role, err := svc.Create(tenantCtx("t1"), contracts.CreateRole{Name: "Admin", SectorScope: authz.ScopeAll})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Get(tenantCtx("t2"), role.ID); !isCode(err, apperror.CodeNotFound) {
		t.Errorf("expected not_found cross-tenant, got %v", err)
	}
}
