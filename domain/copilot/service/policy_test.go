package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeMessages struct{ items []*conventity.Message }

func (r *fakeMessages) Create(context.Context, *conventity.Message) error { return nil }
func (r *fakeMessages) Update(context.Context, *conventity.Message) error { return nil }
func (r *fakeMessages) FindByID(context.Context, string) (*conventity.Message, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeMessages) ListByConversation(context.Context, string, shared.PageRequest) ([]*conventity.Message, error) {
	// newest-first, as the real repo returns.
	return r.items, nil
}
func (r *fakeMessages) LatestByConversation(context.Context, string) (*conventity.Message, error) {
	if len(r.items) == 0 {
		return nil, apperror.NotFound("none")
	}
	return r.items[0], nil
}
func (r *fakeMessages) LatestByConversations(context.Context, []string) (map[string]*conventity.Message, error) {
	return nil, nil
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
func (r *fakeConvRepo) ListInactiveOpen(context.Context, time.Time, int) ([]*conventity.Conversation, error) {
	return nil, nil
}
func (r *fakeConvRepo) List(context.Context, convcontracts.ListFilter, convcontracts.Visibility, shared.PageRequest) ([]*conventity.Conversation, error) {
	return nil, nil
}

type fakeConfigRepo struct{ cfg *entity.AIConfig }

func (r *fakeConfigRepo) Create(context.Context, *entity.AIConfig) error { return nil }
func (r *fakeConfigRepo) Update(context.Context, *entity.AIConfig) error { return nil }
func (r *fakeConfigRepo) FindByTenant(context.Context) (*entity.AIConfig, error) {
	if r.cfg == nil {
		return nil, apperror.NotFound("nf")
	}
	return r.cfg, nil
}

type fakeLogs struct{ entries []*entity.AILog }

func (r *fakeLogs) Create(_ context.Context, l *entity.AILog) error {
	r.entries = append(r.entries, l)
	return nil
}
func (r *fakeLogs) ListByConversation(context.Context, string, shared.PageRequest) ([]*entity.AILog, error) {
	return r.entries, nil
}
func (r *fakeLogs) last() *entity.AILog {
	if len(r.entries) == 0 {
		return nil
	}
	return r.entries[len(r.entries)-1]
}

// data sources that always return data, so the policy is the only gate.
type allowAllCustomer struct{}

func (allowAllCustomer) Customer(context.Context, string) (*contracts.CustomerInfo, error) {
	return &contracts.CustomerInfo{Name: "Jane Doe", Document: "12345678900", Phone: "5511"}, nil
}

type allowAllFinancial struct{}

func (allowAllFinancial) Financial(context.Context, string) (*contracts.FinancialInfo, error) {
	return &contracts.FinancialInfo{Summary: "overdue invoice R$120"}, nil
}

type allowAllMonitoring struct{}

func (allowAllMonitoring) Monitoring(context.Context, string) (*contracts.MonitoringInfo, error) {
	return &contracts.MonitoringInfo{Summary: "offline since 09:00"}, nil
}

// spyProvider captures the context it was asked to infer over.
type spyProvider struct{ got contracts.PromptContext }

func (p *spyProvider) Name() string { return "spy" }
func (p *spyProvider) Infer(_ context.Context, req contracts.Request) (contracts.Response, error) {
	p.got = req.Context
	return contracts.Response{Text: "ok", TokensInput: 5, TokensOutput: 2}, nil
}

type spyResolver struct{ p contracts.AIProvider }

func (r spyResolver) Resolve(entity.Provider) (contracts.AIProvider, error) { return r.p, nil }

// ── helpers ──────────────────────────────────────────────────────────────────

func builderWithAllSources(msgs *fakeMessages) *ContextBuilder {
	return NewContextBuilder(msgs, allowAllCustomer{}, allowAllFinancial{}, allowAllMonitoring{})
}

func sampleMessages() *fakeMessages {
	// newest-first
	return &fakeMessages{items: []*conventity.Message{
		{ID: "m2", Direction: conventity.DirectionOutbound, SenderType: conventity.SenderAgent, Text: "How can I help?"},
		{ID: "m1", Direction: conventity.DirectionInbound, Text: "My internet is down"},
		{ID: "note", Direction: conventity.DirectionInternal, Text: "secret internal note"},
	}}
}

func conv() *conventity.Conversation {
	return &conventity.Conversation{ID: "conv1", TenantID: "t1", Channel: "whatsapp", ContactID: "c1", SectorID: "s1"}
}

// ── context-builder privacy tests ────────────────────────────────────────────

func TestContext_AllPoliciesDisabled_NoEnrichment(t *testing.T) {
	b := builderWithAllSources(sampleMessages())
	cfg := &entity.AIConfig{} // all allow_* false
	pc := b.Build(context.Background(), cfg, conv(), "")

	if pc.Customer != nil {
		t.Errorf("customer data leaked despite allow_customer_data=false: %+v", pc.Customer)
	}
	if pc.Financial != nil {
		t.Errorf("financial data leaked despite allow_financial_data=false: %+v", pc.Financial)
	}
	if pc.Monitoring != nil {
		t.Errorf("monitoring data leaked despite allow_monitoring_data=false: %+v", pc.Monitoring)
	}
	// Transcript is always present, but internal notes must be excluded.
	if len(pc.Transcript) != 2 {
		t.Fatalf("expected 2 non-internal turns, got %d", len(pc.Transcript))
	}
	for _, turn := range pc.Transcript {
		if turn.Text == "secret internal note" {
			t.Errorf("internal note leaked into transcript")
		}
	}
	// chronological order: inbound first.
	if pc.Transcript[0].Role != "customer" || pc.Transcript[1].Role != "agent" {
		t.Errorf("transcript not chronological/role-mapped: %+v", pc.Transcript)
	}
}

func TestContext_FinancialGated(t *testing.T) {
	b := builderWithAllSources(sampleMessages())
	// customer + monitoring allowed, financial NOT.
	cfg := &entity.AIConfig{AllowCustomerData: true, AllowMonitoringData: true}
	pc := b.Build(context.Background(), cfg, conv(), "")

	if pc.Customer == nil {
		t.Errorf("customer should be present when allowed")
	}
	if pc.Monitoring == nil {
		t.Errorf("monitoring should be present when allowed")
	}
	if pc.Financial != nil {
		t.Errorf("financial must be excluded when allow_financial_data=false")
	}
}

func TestContext_AllPoliciesEnabled_FullEnrichment(t *testing.T) {
	b := builderWithAllSources(sampleMessages())
	cfg := &entity.AIConfig{AllowCustomerData: true, AllowFinancialData: true, AllowMonitoringData: true}
	pc := b.Build(context.Background(), cfg, conv(), "")
	if pc.Customer == nil || pc.Financial == nil || pc.Monitoring == nil {
		t.Errorf("all sections should be present when all policies enabled: %+v", pc)
	}
}

func TestContext_NilSourcesAreSafe(t *testing.T) {
	// Policies enabled but no sources wired → no data, no panic.
	b := NewContextBuilder(sampleMessages(), nil, nil, nil)
	cfg := &entity.AIConfig{AllowCustomerData: true, AllowFinancialData: true, AllowMonitoringData: true}
	pc := b.Build(context.Background(), cfg, conv(), "")
	if pc.Customer != nil || pc.Financial != nil || pc.Monitoring != nil {
		t.Errorf("nil sources should yield no enrichment")
	}
}

// ── service-level tests ──────────────────────────────────────────────────────

func newService(cfg *entity.AIConfig, spy *spyProvider) (*Service, *fakeLogs) {
	logs := &fakeLogs{}
	cfgSvc := NewConfigService(&fakeConfigRepo{cfg: cfg}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	convs := &fakeConvRepo{items: map[string]*conventity.Conversation{"conv1": conv()}}
	builder := builderWithAllSources(sampleMessages())
	svc := NewService(cfgSvc, logs, convs, builder, spyResolver{p: spy}, shared.NoopPublisher{}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	return svc, logs
}

func allCtx() context.Context {
	ctx := shared.WithTenant(context.Background(), "t1")
	return authz.WithAuthContext(ctx, authz.NewAuthContext("t1", "u1", nil, nil, authz.ScopeAll))
}

func TestService_SuggestReply_PersistsLogAndRespectsPolicy(t *testing.T) {
	spy := &spyProvider{}
	cfg := &entity.AIConfig{ID: "cfg1", TenantID: "t1", Provider: entity.Provider("echo"), Model: "echo-1", APIKey: "test-key", Enabled: true}
	svc, logs := newService(cfg, spy)

	res, err := svc.SuggestReply(allCtx(), contracts.SuggestReplyInput{ConversationID: "conv1"})
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if res.Action != entity.ActionSuggestReply || res.Text == "" {
		t.Errorf("unexpected result: %+v", res)
	}
	// The provider must NOT have received financial data (policy false).
	if spy.got.Financial != nil || spy.got.Customer != nil {
		t.Errorf("provider received disallowed data: %+v", spy.got)
	}
	// AILog persisted.
	l := logs.last()
	if l == nil || l.Status != entity.StatusSuccess || l.Action != entity.ActionSuggestReply {
		t.Fatalf("expected a success log, got %+v", l)
	}
	if l.TokensInput == 0 || l.OutputSummary == "" {
		t.Errorf("log missing token/output summary: %+v", l)
	}
}

func TestService_HumanApprovalRequired(t *testing.T) {
	spy := &spyProvider{}
	cfg := &entity.AIConfig{ID: "cfg1", TenantID: "t1", Provider: entity.Provider("echo"), Model: "echo-1", APIKey: "test-key", Enabled: true, HumanApprovalRequired: true}
	svc, logs := newService(cfg, spy)

	res, err := svc.Summarize(allCtx(), contracts.SummarizeInput{ConversationID: "conv1"})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if !res.RequiresApproval {
		t.Errorf("expected requires_approval=true")
	}
	if l := logs.last(); l == nil || l.Status != entity.StatusPendingApproval {
		t.Errorf("expected pending_approval log, got %+v", l)
	}
}

func TestService_Disabled(t *testing.T) {
	spy := &spyProvider{}
	cfg := &entity.AIConfig{ID: "cfg1", TenantID: "t1", Provider: entity.Provider("echo"), Enabled: false}
	svc, _ := newService(cfg, spy)
	if _, err := svc.Summarize(allCtx(), contracts.SummarizeInput{ConversationID: "conv1"}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation error when disabled, got %v", err)
	}
}

func TestService_VisibilityEnforced(t *testing.T) {
	spy := &spyProvider{}
	cfg := &entity.AIConfig{ID: "cfg1", TenantID: "t1", Provider: entity.Provider("echo"), Enabled: true}
	svc, _ := newService(cfg, spy)

	// Out-of-scope agent (own scope, different sector, not assigned).
	ctx := shared.WithTenant(context.Background(), "t1")
	ctx = authz.WithAuthContext(ctx, authz.NewAuthContext("t1", "bob", nil, []string{"s2"}, authz.ScopeOwn))
	if _, err := svc.SuggestReply(ctx, contracts.SuggestReplyInput{ConversationID: "conv1"}); apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("expected not_found for out-of-scope conversation, got %v", err)
	}
}
