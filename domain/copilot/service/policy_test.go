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
func (r *fakeConvRepo) FindByIDs(context.Context, []string) ([]*conventity.Conversation, error) {
	return nil, nil
}
func (r *fakeConvRepo) FindLastByContactChannelID(context.Context, string, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindOpenByContactChannelID(context.Context, string, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindOpenByContact(context.Context, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindLastByContact(context.Context, string) (*conventity.Conversation, error) {
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

// spyProvider captures the request it was asked to infer over.
type spyProvider struct {
	got    contracts.PromptContext
	gotReq contracts.Request
}

func (p *spyProvider) Name() string { return "spy" }
func (p *spyProvider) Infer(_ context.Context, req contracts.Request) (contracts.Response, error) {
	p.got = req.Context
	p.gotReq = req
	return contracts.Response{Text: "ok", TokensInput: 5, TokensOutput: 2}, nil
}

type spyResolver struct{ p contracts.AIProvider }

func (r spyResolver) Resolve(entity.Provider) (contracts.AIProvider, error) { return r.p, nil }

// ── helpers ──────────────────────────────────────────────────────────────────

func builderWithAllSources(msgs *fakeMessages) *ContextBuilder {
	return NewContextBuilder(msgs, allowAllCustomer{})
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
	return &conventity.Conversation{ID: "conv1", TenantID: "t1", Channel: "whatsapp", ChannelID: "ch1", ContactID: "c1", SectorID: "s1"}
}

// ── context-builder privacy tests ────────────────────────────────────────────

func TestContext_AllPoliciesDisabled_NoEnrichment(t *testing.T) {
	b := builderWithAllSources(sampleMessages())
	pc := b.Build(context.Background(), entity.Behavior{}, conv(), "", nil) // all gates false

	if pc.Customer != nil {
		t.Errorf("customer data leaked despite allow_customer_data=false: %+v", pc.Customer)
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

func TestContext_CustomerEnrichmentWhenAllowed(t *testing.T) {
	b := builderWithAllSources(sampleMessages())
	beh := entity.Behavior{AllowCustomerData: true}
	pc := b.Build(context.Background(), beh, conv(), "", nil)
	if pc.Customer == nil {
		t.Errorf("customer should be present when allow_customer_data is on: %+v", pc)
	}
}

func TestContext_NilSourcesAreSafe(t *testing.T) {
	// Gate on but no source wired → no data, no panic.
	b := NewContextBuilder(sampleMessages(), nil)
	beh := entity.Behavior{AllowCustomerData: true}
	pc := b.Build(context.Background(), beh, conv(), "", nil)
	if pc.Customer != nil {
		t.Errorf("nil source should yield no enrichment")
	}
}

// ── service-level tests ──────────────────────────────────────────────────────

func newService(cfg *entity.AIConfig, spy *spyProvider, assistants ...*entity.Assistant) (*Service, *fakeLogs) {
	logs := &fakeLogs{}
	cfgSvc := NewConfigService(&fakeConfigRepo{cfg: cfg}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	convs := &fakeConvRepo{items: map[string]*conventity.Conversation{"conv1": conv()}}
	builder := builderWithAllSources(sampleMessages())
	svc := NewService(cfgSvc, logs, convs, builder, spyResolver{p: spy}, shared.NoopPublisher{}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	// Resolve per-assistant behavior; with no assistant the service uses the
	// conservative DefaultBehavior (all gates off).
	repo := &fakeAssistantRepo{byID: map[string]*entity.Assistant{}}
	for _, a := range assistants {
		repo.byID[a.ID] = a
	}
	svc.SetAssistantResolver(repo)
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
	if spy.got.Customer != nil {
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
	cfg := &entity.AIConfig{ID: "cfg1", TenantID: "t1", Provider: entity.Provider("echo"), Model: "echo-1", APIKey: "test-key", Enabled: true}
	// human_approval_required now lives on the assistant serving the channel.
	assistant := &entity.Assistant{ID: "a1", TenantID: "t1", Name: "A", ChannelIDs: []string{"ch1"}, HumanApprovalRequired: true, Enabled: true}
	svc, logs := newService(cfg, spy, assistant)

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

// ── per-assistant behavior tests ─────────────────────────────────────────────

func echoCfg() *entity.AIConfig {
	return &entity.AIConfig{ID: "cfg1", TenantID: "t1", Provider: entity.Provider("echo"), Model: "echo-1", APIKey: "test-key", Enabled: true}
}

func TestService_AssistantGatesEnrichPrompt(t *testing.T) {
	spy := &spyProvider{}
	// The assistant serving ch1 allows customer data and sets a temperature.
	a := &entity.Assistant{ID: "a1", TenantID: "t1", Name: "Vendas", ChannelIDs: []string{"ch1"},
		AllowCustomerData: true, Temperature: 1.2, MaxTokens: 700,
		SystemInstructions: "Persona de vendas.", Enabled: true}
	svc, _ := newService(echoCfg(), spy, a)

	if _, err := svc.Summarize(allCtx(), contracts.SummarizeInput{ConversationID: "conv1"}); err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if spy.got.Customer == nil {
		t.Error("the assistant's allow_customer_data gate must let customer data into the prompt")
	}
	if spy.gotReq.Temperature != 1.2 || spy.gotReq.MaxTokens != 700 {
		t.Errorf("the assistant's sampling must be applied, got temp=%v max=%d", spy.gotReq.Temperature, spy.gotReq.MaxTokens)
	}
	if spy.gotReq.SystemInstructions != "Persona de vendas." {
		t.Errorf("the assistant persona must reach the request, got %q", spy.gotReq.SystemInstructions)
	}
}

func TestService_NoAssistantUsesConservativeDefault(t *testing.T) {
	spy := &spyProvider{}
	svc, _ := newService(echoCfg(), spy) // no assistant for ch1

	if _, err := svc.Summarize(allCtx(), contracts.SummarizeInput{ConversationID: "conv1"}); err != nil {
		t.Fatalf("summarize: %v", err)
	}
	// All data gates OFF, no persona, default sampling.
	if spy.got.Customer != nil {
		t.Errorf("no assistant must gate ALL data, got %+v", spy.got)
	}
	if spy.gotReq.SystemInstructions != "" {
		t.Errorf("no assistant must mean no persona, got %q", spy.gotReq.SystemInstructions)
	}
	if spy.gotReq.Temperature != entity.DefaultTemperature || spy.gotReq.MaxTokens != entity.DefaultMaxTokens {
		t.Errorf("no assistant must use default sampling, got temp=%v max=%d", spy.gotReq.Temperature, spy.gotReq.MaxTokens)
	}
}

func (r *fakeMessages) FindByExternalMessageID(context.Context, string, string) (*conventity.Message, error) {
	return nil, apperror.NotFound("nf")
}
