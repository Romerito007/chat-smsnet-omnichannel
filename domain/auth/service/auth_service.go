// Package service holds the authentication business logic: login, refresh-token
// rotation and logout.
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth/contracts"
	authentity "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/entity"
	authrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam"
	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// errInvalidCredentials is the single, generic error returned for any login
// failure, to avoid leaking whether an email exists.
var errInvalidCredentials = apperror.Unauthorized("invalid credentials")

// Service implements login / refresh / logout.
type Service struct {
	users   iamrepo.UserRepository
	roles   iamrepo.RoleRepository
	tokens  authrepo.RefreshTokenRepository
	hasher  iam.PasswordHasher
	tm      auth.TokenManager
	clock   shared.Clock
	auditor shared.Auditor
}

// New builds the auth service.
func New(
	users iamrepo.UserRepository,
	roles iamrepo.RoleRepository,
	tokens authrepo.RefreshTokenRepository,
	hasher iam.PasswordHasher,
	tm auth.TokenManager,
	clock shared.Clock,
) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{users: users, roles: roles, tokens: tokens, hasher: hasher, tm: tm, clock: clock, auditor: shared.NoopAuditor{}}
}

// SetAuditor wires the audit trail. Optional: when unset, auth events are not
// audited.
func (s *Service) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// Login authenticates a user and issues an access + refresh token pair.
//
// The tenant is resolved from the matched user record (email is looked up across
// tenants since the request is pre-authentication); it is then pinned on the
// context so every subsequent read is tenant-scoped.
func (s *Service) Login(ctx context.Context, cmd contracts.LoginCommand) (*contracts.TokenPair, error) {
	email := strings.ToLower(strings.TrimSpace(cmd.Email))
	if email == "" || cmd.Password == "" {
		return nil, errInvalidCredentials
	}

	user, err := s.users.FindByEmailAnyTenant(ctx, email)
	if err != nil {
		if isNotFound(err) {
			// Compare against a dummy hash to keep timing roughly constant.
			_ = s.hasher.Compare("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinv", cmd.Password)
			return nil, errInvalidCredentials
		}
		return nil, err
	}
	if !user.IsActive() {
		return nil, errInvalidCredentials
	}
	if err := s.hasher.Compare(user.PasswordHash, cmd.Password); err != nil {
		return nil, errInvalidCredentials
	}

	ctx = shared.WithTenant(ctx, user.TenantID)
	pair, err := s.issue(ctx, user, cmd.UserAgent, cmd.IP)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		TenantID: user.TenantID, ActorID: user.ID, ActorType: shared.ActorTypeUser,
		Action: "auth.login", ResourceType: "user", ResourceID: user.ID,
		IP: cmd.IP, UserAgent: cmd.UserAgent,
	})
	return pair, nil
}

// Refresh validates and rotates a refresh token, issuing a new pair.
func (s *Service) Refresh(ctx context.Context, cmd contracts.RefreshCommand) (*contracts.TokenPair, error) {
	if strings.TrimSpace(cmd.RefreshToken) == "" {
		return nil, errInvalidCredentials
	}
	hash := s.tm.HashRefresh(cmd.RefreshToken)
	rt, err := s.tokens.FindByHash(ctx, hash)
	if err != nil {
		if isNotFound(err) {
			return nil, errInvalidCredentials
		}
		return nil, err
	}
	if !rt.Active(s.clock.Now()) {
		return nil, errInvalidCredentials
	}

	// Rotation: revoke the presented token before issuing a new one.
	if err := s.tokens.Revoke(ctx, rt.ID); err != nil {
		return nil, err
	}

	ctx = shared.WithTenant(ctx, rt.TenantID)
	user, err := s.users.FindByID(ctx, rt.UserID)
	if err != nil {
		if isNotFound(err) {
			return nil, errInvalidCredentials
		}
		return nil, err
	}
	if !user.IsActive() {
		return nil, errInvalidCredentials
	}
	return s.issue(ctx, user, cmd.UserAgent, cmd.IP)
}

// Logout revokes the presented refresh token. It is idempotent: an unknown or
// already-revoked token is not an error.
func (s *Service) Logout(ctx context.Context, cmd contracts.LogoutCommand) error {
	if strings.TrimSpace(cmd.RefreshToken) == "" {
		return nil
	}
	hash := s.tm.HashRefresh(cmd.RefreshToken)
	rt, err := s.tokens.FindByHash(ctx, hash)
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return err
	}
	if err := s.tokens.Revoke(ctx, rt.ID); err != nil {
		return err
	}
	_ = s.auditor.Record(shared.WithTenant(ctx, rt.TenantID), shared.AuditEntry{
		TenantID: rt.TenantID, ActorID: rt.UserID, ActorType: shared.ActorTypeUser,
		Action: "auth.logout", ResourceType: "user", ResourceID: rt.UserID,
	})
	return nil
}

// issue resolves the user's effective permissions and mints the token pair,
// persisting the new refresh token. The context must already be tenant-scoped.
func (s *Service) issue(ctx context.Context, user *iamentity.User, userAgent, ip string) (*contracts.TokenPair, error) {
	roles, err := s.roles.FindByIDs(ctx, user.RoleIDs)
	if err != nil {
		return nil, err
	}
	perms, scope := iamservice.ResolveEffective(roles)

	access, accessExp, err := s.tm.IssueAccess(auth.AccessClaims{
		TenantID:    user.TenantID,
		UserID:      user.ID,
		Permissions: perms,
		SectorIDs:   user.SectorIDs,
		SectorScope: scope,
	})
	if err != nil {
		return nil, apperror.Internal("could not issue access token").Wrap(err)
	}

	refreshPlain, refreshExp, err := s.tm.GenerateRefresh()
	if err != nil {
		return nil, apperror.Internal("could not generate refresh token").Wrap(err)
	}

	now := s.clock.Now()
	record := &authentity.RefreshToken{
		ID:        shared.NewID(),
		TenantID:  user.TenantID,
		UserID:    user.ID,
		TokenHash: s.tm.HashRefresh(refreshPlain),
		ExpiresAt: refreshExp,
		UserAgent: userAgent,
		IP:        ip,
		CreatedAt: now,
	}
	if err := s.tokens.Create(ctx, record); err != nil {
		return nil, err
	}

	return &contracts.TokenPair{
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     refreshPlain,
		RefreshExpiresAt: refreshExp,
		User:             user,
		Permissions:      perms,
		SectorScope:      scope,
	}, nil
}

func isNotFound(err error) bool {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr.Code == apperror.CodeNotFound
	}
	return false
}
