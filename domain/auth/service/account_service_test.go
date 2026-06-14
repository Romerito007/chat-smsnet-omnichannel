package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth/contracts"
	authentity "github.com/romerito007/chat-smsnet-omnichannel/domain/auth/entity"
	iamentity "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	tenantentity "github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/entity"
)

// ── account-specific fakes ────────────────────────────────────────────────────

type acctTenantRepo struct {
	byID map[string]*tenantentity.Tenant
}

func (r *acctTenantRepo) Create(_ context.Context, t *tenantentity.Tenant) error {
	r.byID[t.ID] = t
	return nil
}
func (r *acctTenantRepo) FindByID(_ context.Context, id string) (*tenantentity.Tenant, error) {
	if t, ok := r.byID[id]; ok {
		return t, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *acctTenantRepo) FindByExternalRef(context.Context, string) (*tenantentity.Tenant, error) {
	return nil, apperror.NotFound("nf")
}
func (r *acctTenantRepo) Update(_ context.Context, t *tenantentity.Tenant) error {
	r.byID[t.ID] = t
	return nil
}
func (r *acctTenantRepo) ListActive(context.Context) ([]*tenantentity.Tenant, error) { return nil, nil }

type acctUserRepo struct {
	iamrepo.UserRepository
	byID    map[string]*iamentity.User
	byEmail map[string]*iamentity.User
}

func newAcctUserRepo() *acctUserRepo {
	return &acctUserRepo{byID: map[string]*iamentity.User{}, byEmail: map[string]*iamentity.User{}}
}
func (r *acctUserRepo) put(u *iamentity.User) {
	cp := *u
	r.byID[u.ID] = &cp
	r.byEmail[u.Email] = &cp
}
func (r *acctUserRepo) Create(_ context.Context, u *iamentity.User) error {
	if _, ok := r.byEmail[u.Email]; ok {
		return apperror.Conflict("duplicate email")
	}
	r.put(u)
	return nil
}
func (r *acctUserRepo) Update(_ context.Context, u *iamentity.User) error {
	r.put(u)
	return nil
}
func (r *acctUserRepo) FindByID(_ context.Context, id string) (*iamentity.User, error) {
	if u, ok := r.byID[id]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *acctUserRepo) FindByEmail(_ context.Context, email string) (*iamentity.User, error) {
	if u, ok := r.byEmail[email]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *acctUserRepo) FindByEmailAnyTenant(_ context.Context, email string) (*iamentity.User, error) {
	return r.FindByEmail(context.Background(), email)
}

type acctRoleRepo struct {
	iamrepo.RoleRepository
	byName map[string]*iamentity.Role
}

func (r *acctRoleRepo) Create(_ context.Context, role *iamentity.Role) error {
	r.byName[role.Name] = role
	return nil
}
func (r *acctRoleRepo) FindByName(_ context.Context, name string) (*iamentity.Role, error) {
	if role, ok := r.byName[name]; ok {
		return role, nil
	}
	return nil, apperror.NotFound("nf")
}

// generic single-use token store reused by the verification/reset fakes.
type acctVerifyRepo struct {
	byHash map[string]*authentity.EmailVerificationToken
}

func (r *acctVerifyRepo) Create(_ context.Context, t *authentity.EmailVerificationToken) error {
	r.byHash[t.TokenHash] = t
	return nil
}
func (r *acctVerifyRepo) FindByHash(_ context.Context, h string) (*authentity.EmailVerificationToken, error) {
	if t, ok := r.byHash[h]; ok {
		return t, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *acctVerifyRepo) MarkUsed(_ context.Context, id string, at time.Time) error {
	for _, t := range r.byHash {
		if t.ID == id {
			t.UsedAt = &at
		}
	}
	return nil
}
func (r *acctVerifyRepo) InvalidateForUser(_ context.Context, userID string, at time.Time) error {
	for _, t := range r.byHash {
		if t.UserID == userID && t.UsedAt == nil {
			t.UsedAt = &at
		}
	}
	return nil
}

type acctResetRepo struct {
	byHash map[string]*authentity.PasswordResetToken
}

func (r *acctResetRepo) Create(_ context.Context, t *authentity.PasswordResetToken) error {
	r.byHash[t.TokenHash] = t
	return nil
}
func (r *acctResetRepo) FindByHash(_ context.Context, h string) (*authentity.PasswordResetToken, error) {
	if t, ok := r.byHash[h]; ok {
		return t, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *acctResetRepo) MarkUsed(_ context.Context, id string, at time.Time) error {
	for _, t := range r.byHash {
		if t.ID == id {
			t.UsedAt = &at
		}
	}
	return nil
}
func (r *acctResetRepo) InvalidateForUser(_ context.Context, userID string, at time.Time) error {
	for _, t := range r.byHash {
		if t.UserID == userID && t.UsedAt == nil {
			t.UsedAt = &at
		}
	}
	return nil
}

type acctInviteRepo struct {
	byHash map[string]*authentity.Invitation
}

func (r *acctInviteRepo) Create(_ context.Context, i *authentity.Invitation) error {
	r.byHash[i.TokenHash] = i
	return nil
}
func (r *acctInviteRepo) FindByHash(_ context.Context, h string) (*authentity.Invitation, error) {
	if i, ok := r.byHash[h]; ok {
		return i, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *acctInviteRepo) FindPendingByEmail(context.Context, string) (*authentity.Invitation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *acctInviteRepo) MarkAccepted(_ context.Context, id string, at time.Time) error {
	for _, i := range r.byHash {
		if i.ID == id {
			i.AcceptedAt = &at
		}
	}
	return nil
}

type fakeMailer struct {
	calls map[string]contracts.AccountEmail
}

func newFakeMailer() *fakeMailer { return &fakeMailer{calls: map[string]contracts.AccountEmail{}} }
func (m *fakeMailer) SendVerification(_ context.Context, e contracts.AccountEmail) error {
	m.calls["verification"] = e
	return nil
}
func (m *fakeMailer) SendInvite(_ context.Context, e contracts.AccountEmail) error {
	m.calls["invite"] = e
	return nil
}
func (m *fakeMailer) SendPasswordReset(_ context.Context, e contracts.AccountEmail) error {
	m.calls["reset"] = e
	return nil
}
func (m *fakeMailer) SendPasswordResetDone(_ context.Context, e contracts.AccountEmail) error {
	m.calls["reset_done"] = e
	return nil
}

// ── fixture ──────────────────────────────────────────────────────────────────

type acctFixture struct {
	svc     *AccountService
	users   *acctUserRepo
	verify  *acctVerifyRepo
	reset   *acctResetRepo
	invites *acctInviteRepo
	refresh *fakeRefreshRepo
	mailer  *fakeMailer
	now     time.Time
}

func newAcctFixture() acctFixture {
	now := time.Unix(1700000000, 0).UTC()
	users := newAcctUserRepo()
	tenants := &acctTenantRepo{byID: map[string]*tenantentity.Tenant{}}
	roleRepo := &acctRoleRepo{byName: map[string]*iamentity.Role{}}
	roles := iamservice.NewRoleService(roleRepo, fixedClock{t: now})
	verify := &acctVerifyRepo{byHash: map[string]*authentity.EmailVerificationToken{}}
	reset := &acctResetRepo{byHash: map[string]*authentity.PasswordResetToken{}}
	invites := &acctInviteRepo{byHash: map[string]*authentity.Invitation{}}
	refresh := &fakeRefreshRepo{tokens: map[string]*authentity.RefreshToken{}}
	mailer := newFakeMailer()
	svc := NewAccountService(tenants, users, roles, refresh, verify, reset, invites, fakeHasher{}, mailer,
		fixedClock{t: now}, AccountConfig{
			AppBaseURL: "https://app.test", VerificationTTL: 24 * time.Hour,
			ResetTTL: time.Hour, InviteTTL: 72 * time.Hour,
		})
	return acctFixture{svc: svc, users: users, verify: verify, reset: reset, invites: invites, refresh: refresh, mailer: mailer, now: now}
}

func tokenFromLink(link string) string {
	_, tok, _ := strings.Cut(link, "token=")
	return tok
}

func tenantCtx() context.Context { return shared.WithTenant(context.Background(), "t1") }

// ── signup + verification ─────────────────────────────────────────────────────

func TestSignup_CreatesTenantOwnerPendingAndSendsVerification(t *testing.T) {
	fx := newAcctFixture()
	res, err := fx.svc.Signup(context.Background(), contracts.SignupCommand{
		CompanyName: "Acme", OwnerName: "Alice", Email: "Alice@Acme.com", Password: "supersecret",
	})
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	if !res.Created || res.TenantID == "" || res.UserID == "" {
		t.Fatalf("expected a created owner, got %+v", res)
	}
	owner := fx.users.byID[res.UserID]
	if owner.Status != iamentity.StatusPendingVerification {
		t.Errorf("owner must be pending, got %s", owner.Status)
	}
	if owner.Email != "alice@acme.com" {
		t.Errorf("email must be normalized, got %q", owner.Email)
	}
	if len(owner.RoleIDs) != 1 {
		t.Errorf("owner must have the owner role, got %v", owner.RoleIDs)
	}
	if _, ok := fx.mailer.calls["verification"]; !ok {
		t.Fatal("verification email was not sent")
	}

	// The emailed token verifies the account.
	token := tokenFromLink(fx.mailer.calls["verification"].Link)
	if err := fx.svc.VerifyEmail(context.Background(), token); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if fx.users.byID[res.UserID].Status != iamentity.StatusActive {
		t.Errorf("owner must be active after verification")
	}
	// Re-using the same token fails (single use).
	if err := fx.svc.VerifyEmail(context.Background(), token); err == nil {
		t.Error("a used verification token must be rejected")
	}
}

func TestSignup_IdempotentByEmail(t *testing.T) {
	fx := newAcctFixture()
	first, err := fx.svc.Signup(context.Background(), contracts.SignupCommand{
		CompanyName: "Acme", OwnerName: "Alice", Email: "a@acme.com", Password: "supersecret",
	})
	if err != nil || !first.Created {
		t.Fatalf("first signup: %v %+v", err, first)
	}
	second, err := fx.svc.Signup(context.Background(), contracts.SignupCommand{
		CompanyName: "Acme 2", OwnerName: "Alice", Email: "a@acme.com", Password: "supersecret",
	})
	if err != nil {
		t.Fatalf("second signup: %v", err)
	}
	if second.Created {
		t.Error("a duplicate email must not create a second account")
	}
}

func TestSignup_ValidationError(t *testing.T) {
	fx := newAcctFixture()
	_, err := fx.svc.Signup(context.Background(), contracts.SignupCommand{
		CompanyName: "", OwnerName: "", Email: "bad", Password: "short",
	})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation_error, got %v", err)
	}
}

func TestVerifyEmail_ExpiredAndUnknown(t *testing.T) {
	fx := newAcctFixture()
	// Unknown token.
	if err := fx.svc.VerifyEmail(context.Background(), "ghost"); err == nil {
		t.Error("unknown token must be rejected")
	}
	// Expired token inserted directly.
	plain, hash := genToken()
	fx.verify.byHash[hash] = &authentity.EmailVerificationToken{
		ID: "v1", TenantID: "t1", UserID: "u1", TokenHash: hash,
		ExpiresAt: fx.now.Add(-time.Hour), CreatedAt: fx.now.Add(-2 * time.Hour),
	}
	if err := fx.svc.VerifyEmail(context.Background(), plain); err == nil {
		t.Error("expired token must be rejected")
	}
}

// ── forgot / reset ────────────────────────────────────────────────────────────

func TestForgotPassword_NeutralForUnknownEmail(t *testing.T) {
	fx := newAcctFixture()
	if err := fx.svc.ForgotPassword(context.Background(), "nobody@nowhere.com"); err != nil {
		t.Fatalf("forgot must be neutral, got %v", err)
	}
	if _, ok := fx.mailer.calls["reset"]; ok {
		t.Error("no reset email should be sent for an unknown email")
	}
	if len(fx.reset.byHash) != 0 {
		t.Error("no reset token should be created for an unknown email")
	}
}

func TestForgotAndReset_EndToEnd(t *testing.T) {
	fx := newAcctFixture()
	fx.users.put(&iamentity.User{ID: "u1", TenantID: "t1", Name: "Al", Email: "al@acme.com", PasswordHash: "hashed:old", Status: iamentity.StatusActive})

	if err := fx.svc.ForgotPassword(context.Background(), "al@acme.com"); err != nil {
		t.Fatalf("forgot: %v", err)
	}
	mail, ok := fx.mailer.calls["reset"]
	if !ok {
		t.Fatal("reset email not sent")
	}
	token := tokenFromLink(mail.Link)

	if err := fx.svc.ResetPassword(context.Background(), contracts.ResetPasswordCommand{Token: token, NewPassword: "brandnewpass"}); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if fx.users.byID["u1"].PasswordHash != "hashed:brandnewpass" {
		t.Errorf("password was not updated, got %q", fx.users.byID["u1"].PasswordHash)
	}
	if _, ok := fx.mailer.calls["reset_done"]; !ok {
		t.Error("reset confirmation email not sent")
	}
	// Token is single-use.
	if err := fx.svc.ResetPassword(context.Background(), contracts.ResetPasswordCommand{Token: token, NewPassword: "anotherpass"}); err == nil {
		t.Error("a used reset token must be rejected")
	}
}

func TestResetPassword_Expired(t *testing.T) {
	fx := newAcctFixture()
	fx.users.put(&iamentity.User{ID: "u1", TenantID: "t1", Email: "al@acme.com", PasswordHash: "hashed:old", Status: iamentity.StatusActive})
	plain, hash := genToken()
	fx.reset.byHash[hash] = &authentity.PasswordResetToken{
		ID: "p1", TenantID: "t1", UserID: "u1", TokenHash: hash, ExpiresAt: fx.now.Add(-time.Minute), CreatedAt: fx.now.Add(-time.Hour),
	}
	if err := fx.svc.ResetPassword(context.Background(), contracts.ResetPasswordCommand{Token: plain, NewPassword: "brandnewpass"}); err == nil {
		t.Error("expired reset token must be rejected")
	}
}

// ── invite / accept ───────────────────────────────────────────────────────────

func TestInviteAndAccept_EndToEnd(t *testing.T) {
	fx := newAcctFixture()
	inv, err := fx.svc.Invite(tenantCtx(), contracts.InviteCommand{Email: "bob@acme.com", RoleIDs: []string{"r-agent"}, SectorIDs: []string{"s1"}})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if inv.TenantID != "t1" {
		t.Errorf("invitation tenant = %q", inv.TenantID)
	}
	mail, ok := fx.mailer.calls["invite"]
	if !ok {
		t.Fatal("invite email not sent")
	}
	token := tokenFromLink(mail.Link)

	if err := fx.svc.AcceptInvite(context.Background(), contracts.AcceptInviteCommand{Token: token, Name: "Bob", Password: "bobspassword"}); err != nil {
		t.Fatalf("accept: %v", err)
	}
	created, err := fx.users.FindByEmail(tenantCtx(), "bob@acme.com")
	if err != nil {
		t.Fatalf("created user not found: %v", err)
	}
	if created.Status != iamentity.StatusActive || len(created.RoleIDs) != 1 {
		t.Errorf("accepted user wrong: %+v", created)
	}
	// Token is single-use.
	if err := fx.svc.AcceptInvite(context.Background(), contracts.AcceptInviteCommand{Token: token, Name: "Bob", Password: "bobspassword"}); err == nil {
		t.Error("a used invitation must be rejected")
	}
}

func TestInvite_ExistingUserConflicts(t *testing.T) {
	fx := newAcctFixture()
	fx.users.put(&iamentity.User{ID: "u9", TenantID: "t1", Email: "taken@acme.com", Status: iamentity.StatusActive})
	if _, err := fx.svc.Invite(tenantCtx(), contracts.InviteCommand{Email: "taken@acme.com"}); apperror.From(err).Code != apperror.CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}
