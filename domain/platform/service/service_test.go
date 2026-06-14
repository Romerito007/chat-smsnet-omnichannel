package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth"
	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/platform/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	tenantentity "github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/entity"
	tenantrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/repository"
)

// ── fakes (embed the interface; only the used methods are implemented) ───────

type fakeTenantRepo struct {
	tenantrepo.TenantRepository
	byRef map[string]*tenantentity.Tenant
}

func (r *fakeTenantRepo) Create(_ context.Context, t *tenantentity.Tenant) error {
	if r.byRef == nil {
		r.byRef = map[string]*tenantentity.Tenant{}
	}
	cp := *t
	r.byRef[t.ExternalRef] = &cp
	return nil
}
func (r *fakeTenantRepo) FindByExternalRef(_ context.Context, ref string) (*tenantentity.Tenant, error) {
	if t, ok := r.byRef[ref]; ok {
		return t, nil
	}
	return nil, apperror.NotFound("nf")
}

type fakeUserRepo struct {
	iamrepo.UserRepository
	byEmail map[string]*iamentity.User
}

func (r *fakeUserRepo) Create(_ context.Context, u *iamentity.User) error {
	if r.byEmail == nil {
		r.byEmail = map[string]*iamentity.User{}
	}
	cp := *u
	r.byEmail[u.Email] = &cp
	return nil
}
func (r *fakeUserRepo) FindByEmailAnyTenant(_ context.Context, email string) (*iamentity.User, error) {
	if u, ok := r.byEmail[email]; ok {
		return u, nil
	}
	return nil, apperror.NotFound("nf")
}

type fakeRoleRepo struct {
	iamrepo.RoleRepository
	created int
}

func (r *fakeRoleRepo) Create(_ context.Context, _ *iamentity.Role) error {
	r.created++
	return nil
}

type fakeHasher struct{}

func (fakeHasher) Hash(plain string) (string, error) { return "hash:" + plain, nil }
func (fakeHasher) Compare(hash, plain string) error  { return nil }

type fakeTokenManager struct{ issued int }

func (m *fakeTokenManager) IssueAccess(claims auth.AccessClaims) (string, time.Time, error) {
	m.issued++
	return "access-" + claims.TenantID + "-" + claims.UserID, time.Unix(1700000000, 0), nil
}
func (m *fakeTokenManager) VerifyAccess(string) (auth.AccessClaims, error) {
	return auth.AccessClaims{}, nil
}
func (m *fakeTokenManager) GenerateRefresh() (string, time.Time, error) { return "", time.Time{}, nil }
func (m *fakeTokenManager) HashRefresh(string) string                   { return "" }

func newSvc() (*Service, *fakeTenantRepo, *fakeUserRepo, *fakeTokenManager) {
	tr := &fakeTenantRepo{}
	ur := &fakeUserRepo{}
	tm := &fakeTokenManager{}
	roles := iamservice.NewRoleService(&fakeRoleRepo{}, shared.SystemClock{})
	svc := New(tr, ur, roles, fakeHasher{}, tm, shared.SystemClock{})
	return svc, tr, ur, tm
}

func cmd() contracts.ProvisionTenant {
	return contracts.ProvisionTenant{
		TenantName: "Acme", OwnerName: "Jane", OwnerEmail: "jane@acme.com",
		OwnerPassword: "supersecret", ExternalRef: "prov-123", KeyID: "prov1",
	}
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestProvision_CreatesActiveOwnerAndToken(t *testing.T) {
	svc, _, ur, tm := newSvc()

	res, err := svc.Provision(context.Background(), cmd())
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	if !res.Created {
		t.Errorf("expected created=true")
	}
	if res.AccessToken == "" || res.TenantID == "" || res.OwnerID == "" {
		t.Errorf("missing fields in result: %+v", res)
	}
	if tm.issued != 1 {
		t.Errorf("expected one token issued, got %d", tm.issued)
	}
	// The owner must be ACTIVE (trusted provisioner skips email verification), so
	// it can authenticate immediately.
	owner := ur.byEmail["jane@acme.com"]
	if owner == nil || owner.Status != iamentity.StatusActive {
		t.Errorf("owner must be created active, got %+v", owner)
	}
}

func TestProvision_IdempotentByExternalRef(t *testing.T) {
	svc, _, _, tm := newSvc()

	first, err := svc.Provision(context.Background(), cmd())
	if err != nil {
		t.Fatalf("first provision: %v", err)
	}
	second, err := svc.Provision(context.Background(), cmd())
	if err != nil {
		t.Fatalf("retry provision: %v", err)
	}
	if second.Created {
		t.Errorf("retry must not create a second tenant (created=false)")
	}
	if second.TenantID != first.TenantID {
		t.Errorf("retry must return the same tenant: %s vs %s", second.TenantID, first.TenantID)
	}
	if second.AccessToken == "" {
		t.Errorf("retry must still return a usable token")
	}
	if tm.issued != 2 {
		t.Errorf("expected a fresh token on retry (2 total), got %d", tm.issued)
	}
}

func TestProvision_ConflictWhenExternalRefReused(t *testing.T) {
	svc, _, _, _ := newSvc()
	if _, err := svc.Provision(context.Background(), cmd()); err != nil {
		t.Fatalf("first provision: %v", err)
	}
	// Same external_ref but a different owner email → conflict.
	c := cmd()
	c.OwnerEmail = "someone-else@acme.com"
	if _, err := svc.Provision(context.Background(), c); apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("expected conflict for reused external_ref, got %v", err)
	}
}

func TestProvision_Validation(t *testing.T) {
	svc, _, _, _ := newSvc()
	cases := map[string]func(*contracts.ProvisionTenant){
		"missing tenant_name":  func(c *contracts.ProvisionTenant) { c.TenantName = "" },
		"missing owner_email":  func(c *contracts.ProvisionTenant) { c.OwnerEmail = "" },
		"short password":       func(c *contracts.ProvisionTenant) { c.OwnerPassword = "x" },
		"missing external_ref": func(c *contracts.ProvisionTenant) { c.ExternalRef = "" },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			c := cmd()
			mut(&c)
			if _, err := svc.Provision(context.Background(), c); apperror.From(err).Code != apperror.CodeValidation {
				t.Errorf("expected validation_error, got %v", err)
			}
		})
	}
}
