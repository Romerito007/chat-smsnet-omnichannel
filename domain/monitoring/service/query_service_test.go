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
	mcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/contracts"
	mentity "github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeConfigRepo struct {
	cfg *mentity.MonitoringIntegrationConfig
}

func (r *fakeConfigRepo) Create(context.Context, *mentity.MonitoringIntegrationConfig) error {
	return nil
}
func (r *fakeConfigRepo) Update(context.Context, *mentity.MonitoringIntegrationConfig) error {
	return nil
}
func (r *fakeConfigRepo) FindEnabled(context.Context) (*mentity.MonitoringIntegrationConfig, error) {
	if r.cfg == nil {
		return nil, apperror.NotFound("nf")
	}
	return r.cfg, nil
}

type fakeLogs struct{ entries []*mentity.MonitoringQueryLog }

func (r *fakeLogs) Create(_ context.Context, l *mentity.MonitoringQueryLog) error {
	r.entries = append(r.entries, l)
	return nil
}
func (r *fakeLogs) last() *mentity.MonitoringQueryLog {
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
	summary   mcontracts.MonitoringSummary
	incidents []mcontracts.Incident
	err       error
	gotLook   mcontracts.Lookup
}

func (g *fakeGateway) GetSummary(_ context.Context, _ *mentity.MonitoringIntegrationConfig, lk mcontracts.Lookup) (mcontracts.MonitoringSummary, error) {
	g.gotLook = lk
	return g.summary, g.err
}
func (g *fakeGateway) GetIncidents(_ context.Context, _ *mentity.MonitoringIntegrationConfig, lk mcontracts.Lookup) ([]mcontracts.Incident, error) {
	g.gotLook = lk
	return g.incidents, g.err
}
func (g *fakeGateway) Ping(context.Context, *mentity.MonitoringIntegrationConfig) error { return g.err }

type fakeLimiter struct{ allow bool }

func (l fakeLimiter) Allow(context.Context, string) (bool, error) { return l.allow, nil }

// ── fixture ──────────────────────────────────────────────────────────────────

type fixture struct {
	svc     *QueryService
	logs    *fakeLogs
	gateway *fakeGateway
}

func newFixture(limiterAllow bool, gwErr error) fixture {
	cfg := &mentity.MonitoringIntegrationConfig{ID: "cfg1", TenantID: "t1", Enabled: true, BaseURL: "http://api", TimeoutMs: 1000}
	convs := &fakeConvRepo{items: map[string]*conventity.Conversation{
		"conv1": {ID: "conv1", TenantID: "t1", Channel: "whatsapp", ContactID: "c1", SectorID: "s1"},
	}}
	contacts := &fakeContactRepo{byID: map[string]*contactentity.Contact{
		"c1": {ID: "c1", TenantID: "t1", Document: "12345678900", Phone: "5511",
			Identities: []contactentity.ChannelIdentity{{Channel: "whatsapp", ExternalID: "wa-1"}}},
	}}
	logs := &fakeLogs{}
	gw := &fakeGateway{
		summary:   mcontracts.MonitoringSummary{CustomerStatus: mcontracts.StatusOnline, Severity: mcontracts.SeverityNormal, ActiveIncidents: 0},
		incidents: []mcontracts.Incident{{ID: "inc1", Severity: mcontracts.SeverityWarning, Title: "flap"}},
		err:       gwErr,
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

func TestSummary_SuccessLogsAndBuildsLookup(t *testing.T) {
	fx := newFixture(true, nil)
	res, err := fx.svc.Summary(allCtx(), "conv1")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if res.CustomerStatus != mcontracts.StatusOnline {
		t.Errorf("unexpected summary: %+v", res)
	}
	// lookup built from the contact.
	if fx.gateway.gotLook.Document != "12345678900" || fx.gateway.gotLook.ExternalID != "wa-1" {
		t.Errorf("lookup not built from contact: %+v", fx.gateway.gotLook)
	}
	l := fx.logs.last()
	if l == nil || l.Status != mentity.StatusSuccess || l.QueryType != mentity.QuerySummary {
		t.Errorf("expected a success log, got %+v", l)
	}
	// The log must never carry an external payload — only metadata.
	if l != nil && l.ErrorSummary != "" {
		t.Errorf("success log should have no error summary, got %q", l.ErrorSummary)
	}
}

func TestIncidents_Success(t *testing.T) {
	fx := newFixture(true, nil)
	res, err := fx.svc.Incidents(allCtx(), "conv1")
	if err != nil {
		t.Fatalf("incidents: %v", err)
	}
	if len(res) != 1 || res[0].ID != "inc1" {
		t.Errorf("unexpected incidents: %+v", res)
	}
	if l := fx.logs.last(); l == nil || l.Status != mentity.StatusSuccess || l.QueryType != mentity.QueryIncidents {
		t.Errorf("expected an incidents success log, got %+v", l)
	}
}

func TestQuery_RateLimited(t *testing.T) {
	fx := newFixture(false, nil)
	_, err := fx.svc.Summary(allCtx(), "conv1")
	if apperror.From(err).Code != apperror.CodeRateLimited {
		t.Errorf("expected rate_limited, got %v", err)
	}
	if l := fx.logs.last(); l == nil || l.Status != mentity.StatusBlocked {
		t.Errorf("expected a blocked log, got %+v", l)
	}
}

func TestQuery_ExternalFailureFriendlyAndLogged(t *testing.T) {
	fx := newFixture(true, errors.New("connection refused"))
	_, err := fx.svc.Summary(allCtx(), "conv1")
	if apperror.From(err).Code != apperror.CodeIntegrationUnavailable {
		t.Errorf("expected integration_unavailable (friendly), got %v", err)
	}
	if l := fx.logs.last(); l == nil || l.Status != mentity.StatusError {
		t.Errorf("expected an error log, got %+v", l)
	}
}

func TestQuery_TimeoutClassified(t *testing.T) {
	fx := newFixture(true, context.DeadlineExceeded)
	_, _ = fx.svc.Incidents(allCtx(), "conv1")
	if l := fx.logs.last(); l == nil || l.Status != mentity.StatusTimeout {
		t.Errorf("expected a timeout log, got %+v", l)
	}
}

func TestQuery_NoConfig(t *testing.T) {
	logs := &fakeLogs{}
	gw := &fakeGateway{}
	svc := NewQueryService(&fakeConfigRepo{cfg: nil}, logs,
		&fakeConvRepo{items: map[string]*conventity.Conversation{"conv1": {ID: "conv1", TenantID: "t1", ContactID: "c1", SectorID: "s1"}}},
		&fakeContactRepo{byID: map[string]*contactentity.Contact{}}, gw, fakeLimiter{allow: true}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	_, err := svc.Summary(allCtx(), "conv1")
	if apperror.From(err).Code != apperror.CodeIntegrationUnavailable {
		t.Errorf("expected integration_unavailable when not configured, got %v", err)
	}
}

func TestQuery_TenantAndVisibilityEnforced(t *testing.T) {
	fx := newFixture(true, nil)
	// Agent with scope own, in sector s2 (not the conversation's s1, not assigned).
	ctx := shared.WithTenant(context.Background(), "t1")
	ctx = authz.WithAuthContext(ctx, authz.NewAuthContext("t1", "bob", nil, []string{"s2"}, authz.ScopeOwn))
	if _, err := fx.svc.Summary(ctx, "conv1"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("expected not_found for an out-of-scope conversation, got %v", err)
	}

	// Different tenant cannot see it either.
	other := shared.WithTenant(context.Background(), "t2")
	other = authz.WithAuthContext(other, authz.NewAuthContext("t2", "x", nil, nil, authz.ScopeAll))
	if _, err := fx.svc.Summary(other, "conv1"); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("expected not_found cross-tenant, got %v", err)
	}
}
