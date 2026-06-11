package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/mail"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth/contracts"
	authentity "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/entity"
	authrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam"
	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	tenantentity "github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/entity"
	tenantrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/repository"
)

// minPasswordLen is the minimum acceptable password length, matching the IAM
// user service.
const minPasswordLen = 8

// errInvalidToken is the single, generic error for any failed token redemption,
// so callers cannot distinguish "unknown" from "expired" from "already used".
var errInvalidToken = apperror.Unauthorized("invalid or expired token")

// AccountConfig carries the link base URL and single-use token lifetimes.
type AccountConfig struct {
	AppBaseURL      string
	VerificationTTL time.Duration
	ResetTTL        time.Duration
	InviteTTL       time.Duration
}

// AccountService owns the account-lifecycle flows: self-service signup, email
// verification, teammate invitation/acceptance and password reset. It composes
// the tenant, IAM and auth-token repositories and sends real email via the
// Mailer port.
type AccountService struct {
	tenants       tenantrepo.TenantRepository
	users         iamrepo.UserRepository
	roles         *iamservice.RoleService
	refreshTokens authrepo.RefreshTokenRepository
	verifyTokens  authrepo.EmailVerificationTokenRepository
	resetTokens   authrepo.PasswordResetTokenRepository
	invitations   authrepo.InvitationRepository
	hasher        iam.PasswordHasher
	mailer        contracts.Mailer
	clock         shared.Clock
	cfg           AccountConfig
	auditor       shared.Auditor
}

// NewAccountService builds the service.
func NewAccountService(
	tenants tenantrepo.TenantRepository,
	users iamrepo.UserRepository,
	roles *iamservice.RoleService,
	refreshTokens authrepo.RefreshTokenRepository,
	verifyTokens authrepo.EmailVerificationTokenRepository,
	resetTokens authrepo.PasswordResetTokenRepository,
	invitations authrepo.InvitationRepository,
	hasher iam.PasswordHasher,
	mailer contracts.Mailer,
	clock shared.Clock,
	cfg AccountConfig,
) *AccountService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &AccountService{
		tenants: tenants, users: users, roles: roles, refreshTokens: refreshTokens,
		verifyTokens: verifyTokens, resetTokens: resetTokens, invitations: invitations,
		hasher: hasher, mailer: mailer, clock: clock, cfg: cfg, auditor: shared.NoopAuditor{},
	}
}

// SetAuditor wires the audit trail. Optional.
func (s *AccountService) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// ── signup ────────────────────────────────────────────────────────────────────

// Signup provisions a new tenant and its owner user (pending verification) and
// sends a verification email. It is idempotent by email: an already-registered
// address yields a neutral result (Created=false) without leaking existence; a
// still-pending owner simply gets a fresh verification email.
func (s *AccountService) Signup(ctx context.Context, cmd contracts.SignupCommand) (contracts.SignupResult, error) {
	company := strings.TrimSpace(cmd.CompanyName)
	name := strings.TrimSpace(cmd.OwnerName)
	email := normalizeEmail(cmd.Email)

	v := map[string]any{}
	if company == "" {
		v["company_name"] = "is required"
	}
	if name == "" {
		v["owner_name"] = "is required"
	}
	if !validEmail(email) {
		v["email"] = "must be a valid email"
	}
	if len(cmd.Password) < minPasswordLen {
		v["password"] = "must be at least 8 characters"
	}
	if len(v) > 0 {
		return contracts.SignupResult{}, apperror.Validation("validation failed").WithDetails(v)
	}

	// Idempotency: never create a second account for the same email.
	if existing, err := s.users.FindByEmailAnyTenant(ctx, email); err == nil {
		if existing.Status == iamentity.StatusPendingVerification {
			tctx := shared.WithTenant(ctx, existing.TenantID)
			_ = s.issueVerification(tctx, existing) // best-effort resend
		}
		return contracts.SignupResult{Created: false}, nil
	} else if !isNotFound(err) {
		return contracts.SignupResult{}, err
	}

	now := s.clock.Now()
	tenant := &tenantentity.Tenant{
		ID:        shared.NewID(),
		Name:      company,
		Status:    tenantentity.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.tenants.Create(ctx, tenant); err != nil {
		return contracts.SignupResult{}, err
	}
	tctx := shared.WithTenant(ctx, tenant.ID)

	roleIDs, err := s.roles.SeedDefaults(tctx)
	if err != nil {
		return contracts.SignupResult{}, err
	}
	hash, err := s.hasher.Hash(cmd.Password)
	if err != nil {
		return contracts.SignupResult{}, apperror.Internal("could not hash password").Wrap(err)
	}
	owner := &iamentity.User{
		ID:                 shared.NewID(),
		TenantID:           tenant.ID,
		Name:               name,
		Email:              email,
		PasswordHash:       hash,
		Status:             iamentity.StatusPendingVerification,
		RoleIDs:            ownerRoleIDs(roleIDs),
		SectorIDs:          []string{},
		MaxConcurrentChats: 0,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.users.Create(tctx, owner); err != nil {
		return contracts.SignupResult{}, err
	}
	if err := s.issueVerification(tctx, owner); err != nil {
		return contracts.SignupResult{}, err
	}
	_ = s.auditor.Record(tctx, shared.AuditEntry{
		TenantID: tenant.ID, ActorID: owner.ID, ActorType: shared.ActorTypeUser,
		Action: "auth.signup", ResourceType: "user", ResourceID: owner.ID,
		Data: map[string]any{"email": email, "company": company},
	})
	return contracts.SignupResult{Created: true, TenantID: tenant.ID, UserID: owner.ID}, nil
}

// ── email verification ────────────────────────────────────────────────────────

// VerifyEmail redeems a verification token and activates the pending user.
func (s *AccountService) VerifyEmail(ctx context.Context, token string) error {
	rec, err := s.verifyTokens.FindByHash(ctx, hashToken(token))
	if err != nil {
		if isNotFound(err) {
			return errInvalidToken
		}
		return err
	}
	if !rec.Usable(s.clock.Now()) {
		return errInvalidToken
	}
	tctx := shared.WithTenant(ctx, rec.TenantID)
	user, err := s.users.FindByID(tctx, rec.UserID)
	if err != nil {
		return err
	}
	if user.Status == iamentity.StatusPendingVerification {
		user.Status = iamentity.StatusActive
		user.UpdatedAt = s.clock.Now()
		if err := s.users.Update(tctx, user); err != nil {
			return err
		}
	}
	if err := s.verifyTokens.MarkUsed(tctx, rec.ID, s.clock.Now()); err != nil {
		return err
	}
	_ = s.auditor.Record(tctx, shared.AuditEntry{
		TenantID: rec.TenantID, ActorID: user.ID, ActorType: shared.ActorTypeUser,
		Action: "auth.verify_email", ResourceType: "user", ResourceID: user.ID,
	})
	return nil
}

// ResendVerification re-issues a verification email for a still-pending user. The
// response is always neutral: it never reveals whether the email exists.
func (s *AccountService) ResendVerification(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	if !validEmail(email) {
		return nil
	}
	user, err := s.users.FindByEmailAnyTenant(ctx, email)
	if err != nil || user.Status != iamentity.StatusPendingVerification {
		return nil
	}
	tctx := shared.WithTenant(ctx, user.TenantID)
	_ = s.verifyTokens.InvalidateForUser(tctx, user.ID, s.clock.Now())
	return s.issueVerification(tctx, user)
}

// ── password reset ────────────────────────────────────────────────────────────

// ForgotPassword issues a reset token + email. Always neutral (never reveals
// whether the email exists).
func (s *AccountService) ForgotPassword(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	if !validEmail(email) {
		return nil
	}
	user, err := s.users.FindByEmailAnyTenant(ctx, email)
	if err != nil || user.Status == iamentity.StatusDisabled {
		return nil
	}
	tctx := shared.WithTenant(ctx, user.TenantID)
	_ = s.resetTokens.InvalidateForUser(tctx, user.ID, s.clock.Now())

	plaintext, hash := genToken()
	now := s.clock.Now()
	if err := s.resetTokens.Create(tctx, &authentity.PasswordResetToken{
		ID: shared.NewID(), TenantID: user.TenantID, UserID: user.ID, TokenHash: hash,
		ExpiresAt: now.Add(s.cfg.ResetTTL), CreatedAt: now,
	}); err != nil {
		return err
	}
	_ = s.auditor.Record(tctx, shared.AuditEntry{
		TenantID: user.TenantID, ActorID: user.ID, ActorType: shared.ActorTypeUser,
		Action: "auth.password_reset_requested", ResourceType: "user", ResourceID: user.ID,
	})
	return s.mailer.SendPasswordReset(tctx, contracts.AccountEmail{
		To: user.Email, Name: user.Name, Link: s.link("/reset-password", plaintext),
	})
}

// ResetPassword redeems a reset token, sets the new password, revokes all
// sessions and sends a confirmation email.
func (s *AccountService) ResetPassword(ctx context.Context, cmd contracts.ResetPasswordCommand) error {
	if len(cmd.NewPassword) < minPasswordLen {
		return apperror.Validation("password must be at least 8 characters")
	}
	rec, err := s.resetTokens.FindByHash(ctx, hashToken(cmd.Token))
	if err != nil {
		if isNotFound(err) {
			return errInvalidToken
		}
		return err
	}
	if !rec.Usable(s.clock.Now()) {
		return errInvalidToken
	}
	tctx := shared.WithTenant(ctx, rec.TenantID)
	user, err := s.users.FindByID(tctx, rec.UserID)
	if err != nil {
		return err
	}
	hash, err := s.hasher.Hash(cmd.NewPassword)
	if err != nil {
		return apperror.Internal("could not hash password").Wrap(err)
	}
	user.PasswordHash = hash
	// Redeeming the reset proves email control, so a pending account becomes active.
	if user.Status == iamentity.StatusPendingVerification {
		user.Status = iamentity.StatusActive
	}
	user.UpdatedAt = s.clock.Now()
	if err := s.users.Update(tctx, user); err != nil {
		return err
	}
	if err := s.resetTokens.MarkUsed(tctx, rec.ID, s.clock.Now()); err != nil {
		return err
	}
	// Security: invalidate every existing session after a password change.
	_ = s.refreshTokens.RevokeAllForUser(tctx, user.TenantID, user.ID)
	_ = s.auditor.Record(tctx, shared.AuditEntry{
		TenantID: user.TenantID, ActorID: user.ID, ActorType: shared.ActorTypeUser,
		Action: "auth.password_reset", ResourceType: "user", ResourceID: user.ID,
	})
	return s.mailer.SendPasswordResetDone(tctx, contracts.AccountEmail{To: user.Email, Name: user.Name})
}

// ── invitation ────────────────────────────────────────────────────────────────

// Invite creates a single-use invitation for a teammate and emails the accept
// link. Tenant-scoped (admin). Fails if a user with the email already exists.
func (s *AccountService) Invite(ctx context.Context, cmd contracts.InviteCommand) (*authentity.Invitation, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	email := normalizeEmail(cmd.Email)
	if !validEmail(email) {
		return nil, apperror.Validation("must be a valid email").WithDetails(map[string]any{"email": "is invalid"})
	}
	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return nil, apperror.Conflict("a user with this email already exists")
	} else if !isNotFound(err) {
		return nil, err
	}

	plaintext, hash := genToken()
	now := s.clock.Now()
	inviter := ""
	if ac, ok := authz.FromContext(ctx); ok {
		inviter = ac.UserID
	}
	inv := &authentity.Invitation{
		ID: shared.NewID(), TenantID: tenantID, Email: email,
		RoleIDs: cmd.RoleIDs, SectorIDs: cmd.SectorIDs, TokenHash: hash,
		ExpiresAt: now.Add(s.cfg.InviteTTL), InvitedBy: inviter, CreatedAt: now,
	}
	if err := s.invitations.Create(ctx, inv); err != nil {
		return nil, err
	}
	company := s.tenantName(ctx, tenantID)
	if err := s.mailer.SendInvite(ctx, contracts.AccountEmail{
		To: email, Company: company, Link: s.link("/accept-invite", plaintext),
	}); err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "user.invited", ResourceType: "invitation", ResourceID: inv.ID,
		Data: map[string]any{"email": email, "role_ids": cmd.RoleIDs},
	})
	return inv, nil
}

// AcceptInvite redeems an invitation, creating the invited (active) user.
func (s *AccountService) AcceptInvite(ctx context.Context, cmd contracts.AcceptInviteCommand) error {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return apperror.Validation("name is required")
	}
	if len(cmd.Password) < minPasswordLen {
		return apperror.Validation("password must be at least 8 characters")
	}
	rec, err := s.invitations.FindByHash(ctx, hashToken(cmd.Token))
	if err != nil {
		if isNotFound(err) {
			return errInvalidToken
		}
		return err
	}
	if !rec.Usable(s.clock.Now()) {
		return errInvalidToken
	}
	tctx := shared.WithTenant(ctx, rec.TenantID)
	hash, err := s.hasher.Hash(cmd.Password)
	if err != nil {
		return apperror.Internal("could not hash password").Wrap(err)
	}
	now := s.clock.Now()
	user := &iamentity.User{
		ID: shared.NewID(), TenantID: rec.TenantID, Name: name, Email: rec.Email,
		PasswordHash: hash, Status: iamentity.StatusActive,
		RoleIDs: rec.RoleIDs, SectorIDs: rec.SectorIDs, MaxConcurrentChats: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.users.Create(tctx, user); err != nil {
		return err
	}
	if err := s.invitations.MarkAccepted(tctx, rec.ID, now); err != nil {
		return err
	}
	_ = s.auditor.Record(tctx, shared.AuditEntry{
		TenantID: rec.TenantID, ActorID: user.ID, ActorType: shared.ActorTypeUser,
		Action: "auth.invite_accepted", ResourceType: "user", ResourceID: user.ID,
		Data: map[string]any{"email": rec.Email},
	})
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// issueVerification creates a verification token for a (tenant-scoped) user and
// emails the confirmation link.
func (s *AccountService) issueVerification(ctx context.Context, user *iamentity.User) error {
	plaintext, hash := genToken()
	now := s.clock.Now()
	if err := s.verifyTokens.Create(ctx, &authentity.EmailVerificationToken{
		ID: shared.NewID(), TenantID: user.TenantID, UserID: user.ID, TokenHash: hash,
		ExpiresAt: now.Add(s.cfg.VerificationTTL), CreatedAt: now,
	}); err != nil {
		return err
	}
	return s.mailer.SendVerification(ctx, contracts.AccountEmail{
		To: user.Email, Name: user.Name, Company: s.tenantName(ctx, user.TenantID),
		Link: s.link("/verify-email", plaintext),
	})
}

func (s *AccountService) tenantName(ctx context.Context, tenantID string) string {
	if t, err := s.tenants.FindByID(ctx, tenantID); err == nil {
		return t.Name
	}
	return ""
}

func (s *AccountService) link(path, token string) string {
	return strings.TrimRight(s.cfg.AppBaseURL, "/") + path + "?token=" + token
}

// ownerRoleIDs returns the owner role id (if seeded) as a slice.
func ownerRoleIDs(roleIDs map[string]string) []string {
	if id, ok := roleIDs[authz.DefaultRoleOwner]; ok && id != "" {
		return []string{id}
	}
	return []string{}
}

func genToken() (plaintext, hash string) {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	return plaintext, hashToken(plaintext)
}

func hashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func validEmail(email string) bool {
	if email == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}
