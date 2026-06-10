package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/repository"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	phcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeConfigRepo struct {
	cfg *phentity.ProviderIntegrationConfig
}

func (r *fakeConfigRepo) Create(context.Context, *phentity.ProviderIntegrationConfig) error {
	return nil
}
func (r *fakeConfigRepo) Update(context.Context, *phentity.ProviderIntegrationConfig) error {
	return nil
}
func (r *fakeConfigRepo) FindByID(context.Context, string) (*phentity.ProviderIntegrationConfig, error) {
	return r.cfg, nil
}
func (r *fakeConfigRepo) FindEnabled(context.Context) (*phentity.ProviderIntegrationConfig, error) {
	if r.cfg == nil {
		return nil, apperror.NotFound("nf")
	}
	return r.cfg, nil
}
func (r *fakeConfigRepo) List(context.Context, shared.PageRequest) ([]*phentity.ProviderIntegrationConfig, error) {
	return nil, nil
}

type fakeLogs struct{ entries []*phentity.ProviderQueryLog }

func (r *fakeLogs) Create(_ context.Context, l *phentity.ProviderQueryLog) error {
	r.entries = append(r.entries, l)
	return nil
}
func (r *fakeLogs) last() *phentity.ProviderQueryLog {
	if len(r.entries) == 0 {
		return nil
	}
	return r.entries[len(r.entries)-1]
}

type fakeConvRepo struct {
	items map[string]*conventity.Conversation
}

func (r *fakeConvRepo) Create(context.Context, *conventity.Conversation) error { return nil }
func (r *fakeConvRepo) Update(context.Context, *conventity.Conversation) error { return nil }
func (r *fakeConvRepo) FindByID(ctx context.Context, id string) (*conventity.Conversation, error) {
	tenant, _ := shared.TenantFrom(ctx)
	if c, ok := r.items[id]; ok && c.TenantID == tenant {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindOpenByContactChannel(context.Context, string, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) List(context.Context, convcontracts.ListFilter, convcontracts.Visibility, shared.PageRequest) ([]*conventity.Conversation, error) {
	return nil, nil
}

type fakeContactRepo struct {
	contactrepo.ContactRepository
	byID map[string]*contactentity.Contact
}

func (r *fakeContactRepo) FindByID(_ context.Context, id string) (*contactentity.Contact, error) {
	if c, ok := r.byID[id]; ok {
		return c, nil
	}
	return nil, apperror.NotFound("nf")
}

type fakeGateway struct {
	profile phcontracts.CustomerProfile
	fin     phcontracts.FinancialStatus
	ticket  phcontracts.Ticket
	err     error
	gotLook phcontracts.Lookup
}

func (g *fakeGateway) GetCustomerProfile(_ context.Context, _ *phentity.ProviderIntegrationConfig, lk phcontracts.Lookup) (phcontracts.CustomerProfile, error) {
	g.gotLook = lk
	return g.profile, g.err
}
func (g *fakeGateway) GetContracts(context.Context, *phentity.ProviderIntegrationConfig, phcontracts.Lookup) ([]phcontracts.Contract, error) {
	return nil, g.err
}
func (g *fakeGateway) GetFinancialStatus(context.Context, *phentity.ProviderIntegrationConfig, phcontracts.Lookup) (phcontracts.FinancialStatus, error) {
	return g.fin, g.err
}
func (g *fakeGateway) GetConnectionStatus(context.Context, *phentity.ProviderIntegrationConfig, phcontracts.Lookup) (phcontracts.ConnectionStatus, error) {
	return phcontracts.ConnectionStatus{}, g.err
}
func (g *fakeGateway) GetTickets(context.Context, *phentity.ProviderIntegrationConfig, phcontracts.Lookup) ([]phcontracts.Ticket, error) {
	return nil, g.err
}
func (g *fakeGateway) OpenTicket(context.Context, *phentity.ProviderIntegrationConfig, phcontracts.Lookup, phcontracts.OpenTicketInput) (phcontracts.Ticket, error) {
	return g.ticket, g.err
}
func (g *fakeGateway) Ping(context.Context, *phentity.ProviderIntegrationConfig) error { return g.err }

type fakeLimiter struct{ allow bool }

func (l fakeLimiter) Allow(context.Context, string) (bool, error) { return l.allow, nil }

// ── fixture ──────────────────────────────────────────────────────────────────

type fixture struct {
	svc     *QueryService
	logs    *fakeLogs
	gateway *fakeGateway
}

func newFixture(limiterAllow bool, gwErr error) fixture {
	cfg := &phentity.ProviderIntegrationConfig{ID: "cfg1", TenantID: "t1", Enabled: true, BaseURL: "http://api", TimeoutMs: 1000}
	convs := &fakeConvRepo{items: map[string]*conventity.Conversation{
		"conv1": {ID: "conv1", TenantID: "t1", Channel: "whatsapp", ContactID: "c1", SectorID: "s1"},
	}}
	contacts := &fakeContactRepo{byID: map[string]*contactentity.Contact{
		"c1": {ID: "c1", TenantID: "t1", Document: "12345678900", Phone: "5511",
			Identities: []contactentity.ChannelIdentity{{Channel: "whatsapp", ExternalID: "wa-1"}}},
	}}
	logs := &fakeLogs{}
	gw := &fakeGateway{
		profile: phcontracts.CustomerProfile{Name: "Jane", Document: "12345678900"},
		fin:     phcontracts.FinancialStatus{Balance: 10, Overdue: true},
		ticket:  phcontracts.Ticket{ID: "tk1", Subject: "help"},
		err:     gwErr,
	}
	svc := NewQueryService(&fakeConfigRepo{cfg: cfg}, logs, convs, contacts, gw, fakeLimiter{allow: limiterAllow}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	return fixture{svc: svc, logs: logs, gateway: gw}
}

// allCtx is an all-scope actor that can see any conversation.
func allCtx() context.Context {
	ctx := shared.WithTenant(context.Background(), "t1")
	return authz.WithAuthContext(ctx, authz.NewAuthContext("t1", "u1", nil, nil, authz.ScopeAll))
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestCustomerProfile_SuccessLogsAndBuildsLookup(t *testing.T) {
	fx := newFixture(true, nil)
	res, err := fx.svc.CustomerProfile(allCtx(), "conv1")
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if res.Name != "Jane" {
		t.Errorf("unexpected profile: %+v", res)
	}
	// lookup built from the contact.
	if fx.gateway.gotLook.Document != "12345678900" || fx.gateway.gotLook.ExternalID != "wa-1" {
		t.Errorf("lookup not built from contact: %+v", fx.gateway.gotLook)
	}
	if l := fx.logs.last(); l == nil || l.Status != phentity.StatusSuccess || l.QueryType != phentity.QueryCustomerProfile {
		t.Errorf("expected a success log, got %+v", l)
	}
}

func TestQuery_RateLimited(t *testing.T) {
	fx := newFixture(false, nil)
	_, err := fx.svc.CustomerProfile(allCtx(), "conv1")
	if apperror.From(err).Code != apperror.CodeRateLimited {
		t.Errorf("expected rate_limited, got %v", err)
	}
	if l := fx.logs.last(); l == nil || l.Status != phentity.StatusBlocked {
		t.Errorf("expected a blocked log, got %+v", l)
	}
}

func TestQuery_ExternalFailureFriendlyAndLogged(t *testing.T) {
	fx := newFixture(true, errors.New("connection refused"))
	_, err := fx.svc.FinancialStatus(allCtx(), "conv1")
	if apperror.From(err).Code != apperror.CodeIntegrationUnavailable {
		t.Errorf("expected integration_unavailable (friendly), got %v", err)
	}
	if l := fx.logs.last(); l == nil || l.Status != phentity.StatusError {
		t.Errorf("expected an error log, got %+v", l)
	}
}

func TestQuery_TimeoutClassified(t *testing.T) {
	fx := newFixture(true, context.DeadlineExceeded)
	_, _ = fx.svc.Tickets(allCtx(), "conv1")
	if l := fx.logs.last(); l == nil || l.Status != phentity.StatusTimeout {
		t.Errorf("expected a timeout log, got %+v", l)
	}
}

func TestQuery_NoConfig(t *testing.T) {
	fx := newFixture(true, nil)
	fx.svc = NewQueryService(&fakeConfigRepo{cfg: nil}, fx.logs,
		&fakeConvRepo{items: map[string]*conventity.Conversation{"conv1": {ID: "conv1", TenantID: "t1", ContactID: "c1", SectorID: "s1"}}},
		&fakeContactRepo{byID: map[string]*contactentity.Contact{}}, fx.gateway, fakeLimiter{allow: true}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	_, err := fx.svc.CustomerProfile(allCtx(), "conv1")
	if apperror.From(err).Code != apperror.CodeIntegrationUnavailable {
		t.Errorf("expected integration_unavailable when not configured, got %v", err)
	}
}

func TestQuery_TenantAndVisibilityEnforced(t *testing.T) {
	fx := newFixture(true, nil)
	// Agent with scope own, in sector s2 (not the conversation's s1, not assigned).
	ctx := shared.WithTenant(context.Background(), "t1")
	ctx = authz.WithAuthContext(ctx, authz.NewAuthContext("t1", "bob", nil, []string{"s2"}, authz.ScopeOwn))
	if _, err := fx.svc.CustomerProfile(ctx, "conv1"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("expected not_found for an out-of-scope conversation, got %v", err)
	}

	// Different tenant cannot see it either.
	other := shared.WithTenant(context.Background(), "t2")
	other = authz.WithAuthContext(other, authz.NewAuthContext("t2", "x", nil, nil, authz.ScopeAll))
	if _, err := fx.svc.CustomerProfile(other, "conv1"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("expected not_found cross-tenant, got %v", err)
	}
}

func TestOpenTicket_Success(t *testing.T) {
	fx := newFixture(true, nil)
	tk, err := fx.svc.OpenTicket(allCtx(), "conv1", phcontracts.OpenTicketInput{Subject: "down"})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if tk.ID != "tk1" {
		t.Errorf("unexpected ticket: %+v", tk)
	}
	if l := fx.logs.last(); l == nil || l.QueryType != phentity.QueryOpenTicket || l.Status != phentity.StatusSuccess {
		t.Errorf("expected open_ticket success log, got %+v", l)
	}
}
