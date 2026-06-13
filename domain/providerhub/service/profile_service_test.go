package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// fakeProfileRepo is an in-memory repository.ProfileRepository. It enforces the
// at-most-one-default invariant on writes, mirroring the partial-unique index, so
// the service's clear-before-set ordering is actually exercised.
type fakeProfileRepo struct {
	byID map[string]*phentity.ISPProfile
}

func newFakeProfileRepo() *fakeProfileRepo {
	return &fakeProfileRepo{byID: map[string]*phentity.ISPProfile{}}
}

func (r *fakeProfileRepo) defaultCount(except string) int {
	n := 0
	for id, p := range r.byID {
		if id != except && p.IsDefault {
			n++
		}
	}
	return n
}

func (r *fakeProfileRepo) Create(_ context.Context, p *phentity.ISPProfile) error {
	if p.IsDefault && r.defaultCount(p.ID) > 0 {
		return apperror.Conflict("duplicate default")
	}
	cp := *p
	r.byID[p.ID] = &cp
	return nil
}

func (r *fakeProfileRepo) Update(_ context.Context, p *phentity.ISPProfile) error {
	if _, ok := r.byID[p.ID]; !ok {
		return apperror.NotFound("nf")
	}
	if p.IsDefault && r.defaultCount(p.ID) > 0 {
		return apperror.Conflict("duplicate default")
	}
	cp := *p
	r.byID[p.ID] = &cp
	return nil
}

func (r *fakeProfileRepo) Delete(_ context.Context, id string) error {
	if _, ok := r.byID[id]; !ok {
		return apperror.NotFound("nf")
	}
	delete(r.byID, id)
	return nil
}

func (r *fakeProfileRepo) FindByID(_ context.Context, id string) (*phentity.ISPProfile, error) {
	p, ok := r.byID[id]
	if !ok {
		return nil, apperror.NotFound("nf")
	}
	cp := *p
	return &cp, nil
}

func (r *fakeProfileRepo) FindDefault(context.Context) (*phentity.ISPProfile, error) {
	for _, p := range r.byID {
		if p.IsDefault {
			cp := *p
			return &cp, nil
		}
	}
	return nil, apperror.NotFound("nf")
}

func (r *fakeProfileRepo) List(context.Context) ([]*phentity.ISPProfile, error) {
	out := make([]*phentity.ISPProfile, 0, len(r.byID))
	for _, p := range r.byID {
		cp := *p
		out = append(out, &cp)
	}
	return out, nil
}

func (r *fakeProfileRepo) ClearDefault(context.Context) error {
	for id, p := range r.byID {
		if p.IsDefault {
			p.IsDefault = false
			r.byID[id] = p
		}
	}
	return nil
}

func profileCtx() context.Context { return shared.WithTenant(context.Background(), "t1") }

func newProfileSvc(repo *fakeProfileRepo) *ProfileService {
	svc := NewProfileService(repo, &fakeGateway{}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	svc.SetEnvDefault("http://gw", "k")
	return svc
}

// validCreds returns a credential map matching the catalog for ixcsoft.
func validCreds() map[string]string {
	return map[string]string{"ixcsoft_host": "h", "ixcsoft_token": "t"}
}

func TestProfileCreate_FirstIsForcedDefault(t *testing.T) {
	svc := newProfileSvc(newFakeProfileRepo())
	p, err := svc.Create(profileCtx(), contracts.CreateProfile{
		Label: "IXC matriz", ISPType: "ixcsoft", Credentials: validCreds(), IsDefault: false,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !p.IsDefault {
		t.Errorf("first profile must be forced default")
	}
	if !p.Enabled {
		t.Errorf("enabled should default to true")
	}
	if p.TimeoutMs != defaultTimeoutMs {
		t.Errorf("timeout default = %d, want %d", p.TimeoutMs, defaultTimeoutMs)
	}
}

func TestProfileCreate_SecondDefaultUnsetsFirst(t *testing.T) {
	repo := newFakeProfileRepo()
	svc := newProfileSvc(repo)
	first, _ := svc.Create(profileCtx(), contracts.CreateProfile{Label: "A", ISPType: "ixcsoft", Credentials: validCreds()})
	second, err := svc.Create(profileCtx(), contracts.CreateProfile{
		Label: "B", ISPType: "mkauth", Credentials: map[string]string{"mkauth_host": "h", "mkauth_token": "t"}, IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if !second.IsDefault {
		t.Errorf("second profile should be default")
	}
	got, _ := repo.FindByID(profileCtx(), first.ID)
	if got.IsDefault {
		t.Errorf("first profile must no longer be default")
	}
	if repo.defaultCount("") != 1 {
		t.Errorf("exactly one default expected, got %d", repo.defaultCount(""))
	}
}

func TestProfileCreate_RejectsUnknownISPAndLegacy(t *testing.T) {
	svc := newProfileSvc(newFakeProfileRepo())
	for _, slug := range []string{"nope", "voalle", "sgp"} {
		_, err := svc.Create(profileCtx(), contracts.CreateProfile{Label: "X", ISPType: slug, Credentials: map[string]string{}})
		if apperror.From(err).Code != apperror.CodeValidation {
			t.Errorf("isp_type %q should be rejected for a profile, got %v", slug, err)
		}
	}
}

func TestProfileCreate_RejectsCredentialKeyMismatch(t *testing.T) {
	svc := newProfileSvc(newFakeProfileRepo())
	// rbxsoft requires host+token+appkey; omit appkey and add an unknown key.
	_, err := svc.Create(profileCtx(), contracts.CreateProfile{
		Label: "RBX", ISPType: "rbxsoft",
		Credentials: map[string]string{"rbxsoft_host": "h", "rbxsoft_token": "t", "rbxsoft_bogus": "x"},
	})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation error for credential mismatch, got %v", err)
	}
}

func TestProfileCreate_AcceptsRbxsoftWithAppkey(t *testing.T) {
	svc := newProfileSvc(newFakeProfileRepo())
	_, err := svc.Create(profileCtx(), contracts.CreateProfile{
		Label: "RBX", ISPType: "rbxsoft",
		Credentials: map[string]string{"rbxsoft_host": "h", "rbxsoft_token": "t", "rbxsoft_appkey": "a"},
	})
	if err != nil {
		t.Fatalf("rbxsoft with appkey should be valid: %v", err)
	}
}

func TestProfileSetDefault_MovesDefault(t *testing.T) {
	repo := newFakeProfileRepo()
	svc := newProfileSvc(repo)
	a, _ := svc.Create(profileCtx(), contracts.CreateProfile{Label: "A", ISPType: "ixcsoft", Credentials: validCreds()})
	b, _ := svc.Create(profileCtx(), contracts.CreateProfile{Label: "B", ISPType: "mkauth", Credentials: map[string]string{"mkauth_host": "h", "mkauth_token": "t"}})

	if _, err := svc.SetDefault(profileCtx(), b.ID); err != nil {
		t.Fatalf("set default: %v", err)
	}
	ga, _ := repo.FindByID(profileCtx(), a.ID)
	gb, _ := repo.FindByID(profileCtx(), b.ID)
	if ga.IsDefault || !gb.IsDefault {
		t.Errorf("default not moved: a.default=%v b.default=%v", ga.IsDefault, gb.IsDefault)
	}
	if repo.defaultCount("") != 1 {
		t.Errorf("exactly one default expected, got %d", repo.defaultCount(""))
	}
}

func TestProfileUpdate_RevalidatesCredentialsOnTypeChange(t *testing.T) {
	repo := newFakeProfileRepo()
	svc := newProfileSvc(repo)
	p, _ := svc.Create(profileCtx(), contracts.CreateProfile{Label: "A", ISPType: "ixcsoft", Credentials: validCreds()})

	// Switch to whmcs but keep ixcsoft credentials → mismatch.
	whmcs := "whmcs"
	_, err := svc.Update(profileCtx(), p.ID, contracts.UpdateProfile{ISPType: &whmcs})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Fatalf("expected validation error when type/credentials mismatch, got %v", err)
	}
}

func TestProfileGatewayStatus_ReportsEnvAndProfiles(t *testing.T) {
	repo := newFakeProfileRepo()
	svc := newProfileSvc(repo)
	p, _ := svc.Create(profileCtx(), contracts.CreateProfile{Label: "A", ISPType: "ixcsoft", Credentials: validCreds()})

	st, err := svc.GatewayStatus(profileCtx())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Source != "env" || !st.Configured {
		t.Errorf("expected env/configured, got %+v", st)
	}
	if !st.HasProfiles || st.ProfilesCount != 1 || st.DefaultProfileID != p.ID {
		t.Errorf("profile summary wrong: %+v", st)
	}
}

func TestProfileGatewayStatus_NoneWithoutEnv(t *testing.T) {
	svc := NewProfileService(newFakeProfileRepo(), &fakeGateway{}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	st, err := svc.GatewayStatus(profileCtx())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Source != "none" || st.Configured {
		t.Errorf("expected none/unconfigured without env, got %+v", st)
	}
}

func TestProfile_RequireTenant(t *testing.T) {
	svc := newProfileSvc(newFakeProfileRepo())
	if _, err := svc.List(context.Background()); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expected forbidden without tenant, got %v", err)
	}
}
