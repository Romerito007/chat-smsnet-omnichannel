// Package service holds the platform-plane provisioning logic: create a tenant +
// its owner and return a ready-to-use tenant-scoped token. It operates ABOVE
// tenant isolation (no incoming tenant context) and does exactly one thing —
// provision — so a leaked platform key can create new tenants but never touch an
// existing one.
package service

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam"
	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/platform/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	tenantentity "github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/entity"
	tenantrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/repository"
)

const minPasswordLen = 8

// Service provisions tenants on behalf of the external provisioner.
type Service struct {
	tenants tenantrepo.TenantRepository
	users   iamrepo.UserRepository
	roles   *iamservice.RoleService
	hasher  iam.PasswordHasher
	tm      auth.TokenManager
	clock   shared.Clock
	auditor shared.Auditor
}

// New builds the service.
func New(
	tenants tenantrepo.TenantRepository,
	users iamrepo.UserRepository,
	roles *iamservice.RoleService,
	hasher iam.PasswordHasher,
	tm auth.TokenManager,
	clock shared.Clock,
) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{tenants: tenants, users: users, roles: roles, hasher: hasher, tm: tm, clock: clock, auditor: shared.NoopAuditor{}}
}

// SetAuditor wires the audit trail. Optional.
func (s *Service) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// Provision creates a tenant + active owner and returns a ready tenant-scoped
// access token. It is idempotent by ExternalRef: a repeat returns the existing
// tenant with a freshly issued token (Created=false), surviving Redis eviction.
func (s *Service) Provision(ctx context.Context, cmd contracts.ProvisionTenant) (contracts.ProvisionResult, error) {
	tenantName := strings.TrimSpace(cmd.TenantName)
	ownerName := strings.TrimSpace(cmd.OwnerName)
	email := strings.ToLower(strings.TrimSpace(cmd.OwnerEmail))
	ref := strings.TrimSpace(cmd.ExternalRef)

	v := map[string]any{}
	if tenantName == "" {
		v["tenant_name"] = "is required"
	}
	if ownerName == "" {
		v["owner_name"] = "is required"
	}
	if !validEmail(email) {
		v["owner_email"] = "must be a valid email"
	}
	if len(cmd.OwnerPassword) < minPasswordLen {
		v["owner_password"] = "must be at least 8 characters"
	}
	if ref == "" {
		v["external_ref"] = "is required"
	}
	if len(v) > 0 {
		return contracts.ProvisionResult{}, apperror.Validation("validation failed").WithDetails(v)
	}

	// Durable idempotency: a repeat for the same external_ref returns the existing
	// tenant + a fresh token instead of creating a duplicate.
	if existing, err := s.tenants.FindByExternalRef(ctx, ref); err == nil {
		owner, oerr := s.users.FindByEmailAnyTenant(ctx, email)
		if oerr != nil || owner.TenantID != existing.ID {
			return contracts.ProvisionResult{}, apperror.Conflict("external_ref already used by a different account")
		}
		token, exp, terr := s.issueOwnerToken(existing.ID, owner.ID)
		if terr != nil {
			return contracts.ProvisionResult{}, terr
		}
		return contracts.ProvisionResult{
			TenantID: existing.ID, TenantName: existing.Name,
			OwnerID: owner.ID, OwnerEmail: owner.Email,
			AccessToken: token, AccessExpiresAt: exp, Created: false,
		}, nil
	} else if apperror.From(err).Code != apperror.CodeNotFound {
		return contracts.ProvisionResult{}, err
	}

	now := s.clock.Now()
	tenant := &tenantentity.Tenant{
		ID:          shared.NewID(),
		Name:        tenantName,
		Status:      tenantentity.StatusActive,
		ExternalRef: ref,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.tenants.Create(ctx, tenant); err != nil {
		return contracts.ProvisionResult{}, err
	}
	tctx := shared.WithTenant(ctx, tenant.ID)

	roleIDs, err := s.roles.SeedDefaults(tctx)
	if err != nil {
		return contracts.ProvisionResult{}, err
	}
	hash, err := s.hasher.Hash(cmd.OwnerPassword)
	if err != nil {
		return contracts.ProvisionResult{}, apperror.Internal("could not hash password").Wrap(err)
	}
	owner := &iamentity.User{
		ID:                 shared.NewID(),
		TenantID:           tenant.ID,
		Name:               ownerName,
		Email:              email,
		PasswordHash:       hash,
		Status:             iamentity.StatusActive, // trusted provisioner: no email verification
		RoleIDs:            ownerRoleIDs(roleIDs),
		SectorIDs:          []string{},
		MaxConcurrentChats: 0,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.users.Create(tctx, owner); err != nil {
		return contracts.ProvisionResult{}, err
	}

	token, exp, err := s.issueOwnerToken(tenant.ID, owner.ID)
	if err != nil {
		return contracts.ProvisionResult{}, err
	}

	_ = s.auditor.Record(tctx, shared.AuditEntry{
		TenantID: tenant.ID, ActorID: cmd.KeyID, ActorType: shared.ActorTypePlatform,
		Action: "platform.tenant_provisioned", ResourceType: "tenant", ResourceID: tenant.ID,
		Data: map[string]any{"external_ref": ref, "owner_email": email, "tenant_name": tenantName},
	})

	return contracts.ProvisionResult{
		TenantID: tenant.ID, TenantName: tenant.Name,
		OwnerID: owner.ID, OwnerEmail: owner.Email,
		AccessToken: token, AccessExpiresAt: exp, Created: true,
	}, nil
}

// issueOwnerToken mints an access token for the owner. The owner is the tenant
// superuser (DefaultRoleOwner = AllPermissions + ScopeAll), so the claims mirror
// exactly what a normal login would resolve — the rest of provisioning then uses
// the ordinary tenant-scoped API.
func (s *Service) issueOwnerToken(tenantID, ownerID string) (string, time.Time, error) {
	token, exp, err := s.tm.IssueAccess(auth.AccessClaims{
		TenantID:    tenantID,
		UserID:      ownerID,
		Permissions: authz.AllPermissions(),
		SectorIDs:   []string{},
		SectorScope: authz.ScopeAll,
	})
	if err != nil {
		return "", time.Time{}, apperror.Internal("could not issue access token").Wrap(err)
	}
	return token, exp, nil
}

func ownerRoleIDs(roleIDs map[string]string) []string {
	if id, ok := roleIDs[authz.DefaultRoleOwner]; ok && id != "" {
		return []string{id}
	}
	return []string{}
}

func validEmail(email string) bool {
	if email == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}
