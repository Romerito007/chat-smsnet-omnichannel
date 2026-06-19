package service

import (
	"context"
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
	cliente    phcontracts.ClienteResult
	planos     []phcontracts.Plano
	empresa    phcontracts.Empresa
	liber      phcontracts.Liberacao
	chamado    phcontracts.Chamado
	err        error
	gotReq     phcontracts.ConsultaClienteRequest
	gotConfig  *phentity.ProviderIntegrationConfig
	gotIdemKey string
}

func (g *fakeGateway) ConsultarCliente(_ context.Context, cfg *phentity.ProviderIntegrationConfig, req phcontracts.ConsultaClienteRequest) (phcontracts.ClienteResult, error) {
	g.gotConfig = cfg
	g.gotReq = req
	return g.cliente, g.err
}
func (g *fakeGateway) ListarPlanos(_ context.Context, cfg *phentity.ProviderIntegrationConfig) ([]phcontracts.Plano, error) {
	g.gotConfig = cfg
	return g.planos, g.err
}
func (g *fakeGateway) DadosEmpresa(_ context.Context, cfg *phentity.ProviderIntegrationConfig) (phcontracts.Empresa, error) {
	g.gotConfig = cfg
	return g.empresa, g.err
}
func (g *fakeGateway) LiberarAcesso(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, _ string) (phcontracts.Liberacao, error) {
	g.gotConfig = cfg
	g.gotIdemKey = phcontracts.IdempotencyKeyFrom(ctx)
	return g.liber, g.err
}
func (g *fakeGateway) AbrirChamado(ctx context.Context, cfg *phentity.ProviderIntegrationConfig, _, _, _ string) (phcontracts.Chamado, error) {
	g.gotConfig = cfg
	g.gotIdemKey = phcontracts.IdempotencyKeyFrom(ctx)
	return g.chamado, g.err
}
func (g *fakeGateway) Ping(context.Context, *phentity.ProviderIntegrationConfig) error { return g.err }

type fakeLimiter struct{ allow bool }

func (l fakeLimiter) Allow(context.Context, string) (bool, error) { return l.allow, nil }

type fakeAuditor struct{ entries []shared.AuditEntry }

func (a *fakeAuditor) Record(_ context.Context, e shared.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}
func (a *fakeAuditor) has(action string) (shared.AuditEntry, bool) {
	for _, e := range a.entries {
		if e.Action == action {
			return e, true
		}
	}
	return shared.AuditEntry{}, false
}

// ── fixture ──────────────────────────────────────────────────────────────────

func actorCtx(tenant, user string, perms ...authz.Permission) context.Context {
	ctx := shared.WithTenant(context.Background(), tenant)
	return authz.WithAuthContext(ctx, authz.NewAuthContext(tenant, user, perms, nil, authz.ScopeAll))
}

func newSvc(gw *fakeGateway, limiterAllow, withProfile bool) (*QueryService, *fakeLogs, *fakeAuditor) {
	repo := newFakeProfileRepo()
	if withProfile {
		repo.byID["cfg1"] = &phentity.ISPProfile{
			ID: "cfg1", TenantID: "t1", Label: "default", ISPType: phentity.ISPHubsoft,
			IsDefault: true, Enabled: true, TimeoutMs: 1000,
			Transports:  []string{phentity.TransportHTTP},
			Credentials: map[string]string{"hubsoft_host": "h"},
		}
	}
	logs := &fakeLogs{}
	aud := &fakeAuditor{}
	svc := NewQueryService(
		repo,
		logs,
		&fakeConvRepo{items: map[string]*conventity.Conversation{
			"cv1": {ID: "cv1", TenantID: "t1", ContactID: "c1", SectorID: "s1"},
		}},
		&fakeContactRepo{byID: map[string]*contactentity.Contact{
			"c1": {ID: "c1", TenantID: "t1", Document: "123", Phone: "+55"},
		}},
		gw,
		fakeLimiter{allow: limiterAllow},
		fixedClock{t: time.Unix(1700000000, 0).UTC()},
	)
	svc.SetEnvDefault("http://api", "k")
	svc.SetAuditor(aud)
	return svc, logs, aud
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestConsultarCliente_SuccessOmitsFaturasWithoutFinancialPermission(t *testing.T) {
	gw := &fakeGateway{cliente: phcontracts.ClienteResult{Cliente: &phcontracts.Cliente{
		Nome: "João", CpfCnpj: "123", Faturas: []phcontracts.Fatura{{Valor: 99.9, Vencimento: "2026-07-01"}},
	}}}
	svc, logs, _ := newSvc(gw, true, true)

	res, err := svc.ConsultarCliente(actorCtx("t1", "u1", authz.IntegrationRead), "cv1", phcontracts.ConsultaClienteRequest{CpfCnpj: "123"})
	if err != nil {
		t.Fatalf("consulta: %v", err)
	}
	if res.Cliente == nil || res.Cliente.Nome != "João" {
		t.Fatalf("expected cliente, got %+v", res)
	}
	if res.Cliente.Faturas != nil {
		t.Errorf("faturas must be omitted without contact.view_financial, got %+v", res.Cliente.Faturas)
	}
	if logs.last() == nil || logs.last().Status != phentity.StatusSuccess {
		t.Errorf("expected a success query log")
	}
}

func TestConsultarCliente_KeepsFaturasWithFinancialPermission(t *testing.T) {
	gw := &fakeGateway{cliente: phcontracts.ClienteResult{Cliente: &phcontracts.Cliente{
		Nome: "João", Faturas: []phcontracts.Fatura{{Valor: 99.9}},
	}}}
	svc, _, _ := newSvc(gw, true, true)

	res, err := svc.ConsultarCliente(actorCtx("t1", "u1", authz.IntegrationRead, authz.ContactViewFinancial), "cv1", phcontracts.ConsultaClienteRequest{CpfCnpj: "123"})
	if err != nil {
		t.Fatalf("consulta: %v", err)
	}
	if len(res.Cliente.Faturas) != 1 {
		t.Errorf("faturas must be kept with contact.view_financial, got %+v", res.Cliente.Faturas)
	}
}

func TestConsultarCliente_NeedsInputReturnsOptions(t *testing.T) {
	gw := &fakeGateway{cliente: phcontracts.ClienteResult{NeedsSelection: true, Options: []phcontracts.ContratoOption{
		{IDCliente: "A", Label: "Contrato A"}, {IDCliente: "B", Label: "Contrato B"},
	}}}
	svc, _, _ := newSvc(gw, true, true)

	res, err := svc.ConsultarCliente(actorCtx("t1", "u1", authz.IntegrationRead), "cv1", phcontracts.ConsultaClienteRequest{Phone: "+55"})
	if err != nil {
		t.Fatalf("consulta: %v", err)
	}
	if !res.NeedsSelection || len(res.Options) != 2 {
		t.Errorf("expected needs_input with 2 options, got %+v", res)
	}
}

func TestConsultarCliente_FillsRequestFromContactWhenEmpty(t *testing.T) {
	gw := &fakeGateway{cliente: phcontracts.ClienteResult{Cliente: &phcontracts.Cliente{Nome: "X"}}}
	svc, _, _ := newSvc(gw, true, true)
	if _, err := svc.ConsultarCliente(actorCtx("t1", "u1", authz.IntegrationRead), "cv1", phcontracts.ConsultaClienteRequest{}); err != nil {
		t.Fatalf("consulta: %v", err)
	}
	if gw.gotReq.CpfCnpj != "123" || gw.gotReq.Phone != "+55" {
		t.Errorf("request not filled from contact: %+v", gw.gotReq)
	}
}

func TestConsultarCliente_NotFoundPropagates(t *testing.T) {
	gw := &fakeGateway{err: apperror.NotFound("cliente não localizado")}
	svc, logs, _ := newSvc(gw, true, true)
	_, err := svc.ConsultarCliente(actorCtx("t1", "u1", authz.IntegrationRead), "cv1", phcontracts.ConsultaClienteRequest{CpfCnpj: "999"})
	if apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("expected not_found, got %v", err)
	}
	if logs.last().Status != phentity.StatusNotFound {
		t.Errorf("log status = %q, want not_found", logs.last().Status)
	}
}

func TestConsultarCliente_FallbackIsIntegrationError(t *testing.T) {
	gw := &fakeGateway{err: apperror.Integration("unavailable")}
	svc, logs, _ := newSvc(gw, true, true)
	_, err := svc.ConsultarCliente(actorCtx("t1", "u1", authz.IntegrationRead), "cv1", phcontracts.ConsultaClienteRequest{CpfCnpj: "1"})
	if apperror.From(err).Code != apperror.CodeIntegrationUnavailable {
		t.Fatalf("expected integration error, got %v", err)
	}
	if logs.last().Status != phentity.StatusError {
		t.Errorf("log status = %q, want error", logs.last().Status)
	}
}

func TestLiberarAcesso_SuccessIsAudited(t *testing.T) {
	gw := &fakeGateway{liber: phcontracts.Liberacao{Liberado: true, Protocolo: "P1"}}
	svc, _, aud := newSvc(gw, true, true)
	res, err := svc.LiberarAcesso(actorCtx("t1", "u1", authz.IntegrationExecuteAction), "cv1", "", "idc1", "idem-1")
	if err != nil {
		t.Fatalf("liberar: %v", err)
	}
	if !res.Liberado || res.Protocolo != "P1" {
		t.Errorf("unexpected liberacao: %+v", res)
	}
	e, ok := aud.has("providerhub.liberacao")
	if !ok {
		t.Fatalf("liberacao not audited: %+v", aud.entries)
	}
	if e.Data["id_cliente"] != "idc1" {
		t.Errorf("audit missing id_cliente: %+v", e)
	}
}

func TestAbrirChamado_SuccessIsAudited(t *testing.T) {
	gw := &fakeGateway{chamado: phcontracts.Chamado{Protocolo: "C1"}}
	svc, _, aud := newSvc(gw, true, true)
	res, err := svc.AbrirChamado(actorCtx("t1", "u1", authz.IntegrationExecuteAction), "cv1", "", "idc1", "sem internet", "detalhe", "idem-1")
	if err != nil {
		t.Fatalf("chamado: %v", err)
	}
	if res.Protocolo != "C1" {
		t.Errorf("unexpected chamado: %+v", res)
	}
	if _, ok := aud.has("providerhub.chamado"); !ok {
		t.Fatalf("chamado not audited: %+v", aud.entries)
	}
}

func TestLiberarAcesso_RequiresIDCliente(t *testing.T) {
	svc, _, _ := newSvc(&fakeGateway{}, true, true)
	_, err := svc.LiberarAcesso(actorCtx("t1", "u1", authz.IntegrationExecuteAction), "cv1", "", "  ", "idem-1")
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation for empty id_cliente, got %v", err)
	}
}

func TestExecute_RateLimited(t *testing.T) {
	svc, logs, _ := newSvc(&fakeGateway{}, false, true)
	_, err := svc.ListarPlanos(actorCtx("t1", "u1", authz.IntegrationRead), "cv1", "")
	if apperror.From(err).Code != apperror.CodeRateLimited {
		t.Fatalf("expected rate_limited, got %v", err)
	}
	if logs.last().Status != phentity.StatusBlocked {
		t.Errorf("log status = %q, want blocked", logs.last().Status)
	}
}

func TestExecute_NoProfile(t *testing.T) {
	// No ISP profile for the tenant → clear conflict (not a 500), external actions off.
	svc, _, _ := newSvc(&fakeGateway{}, true, false)
	_, err := svc.DadosEmpresa(actorCtx("t1", "u1", authz.IntegrationRead), "cv1", "")
	if apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("expected conflict without any ISP profile, got %v", err)
	}
}

func TestResolve_AmbiguousReturnsSelectionSentinel(t *testing.T) {
	// Two enabled profiles, none default → ambiguous → needs_isp_selection sentinel.
	gw := &fakeGateway{}
	repo := newFakeProfileRepo()
	repo.byID["a"] = &phentity.ISPProfile{ID: "a", TenantID: "t1", Label: "A", ISPType: phentity.ISPIXCSoft, Enabled: true, Transports: []string{phentity.TransportHTTP}}
	repo.byID["b"] = &phentity.ISPProfile{ID: "b", TenantID: "t1", Label: "B", ISPType: phentity.ISPMKAuth, Enabled: true, Transports: []string{phentity.TransportHTTP}}
	svc := NewQueryService(repo, &fakeLogs{}, &fakeConvRepo{items: map[string]*conventity.Conversation{
		"cv1": {ID: "cv1", TenantID: "t1", ContactID: "c1", SectorID: "s1"},
	}}, &fakeContactRepo{byID: map[string]*contactentity.Contact{"c1": {ID: "c1", TenantID: "t1"}}},
		gw, fakeLimiter{allow: true}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	svc.SetEnvDefault("http://api", "k")

	_, err := svc.DadosEmpresa(actorCtx("t1", "u1", authz.IntegrationRead), "cv1", "")
	sel, ok := AsISPSelectionRequired(err)
	if !ok {
		t.Fatalf("expected ISP selection sentinel, got %v", err)
	}
	if len(sel.Eligible) != 2 {
		t.Errorf("expected 2 eligible profiles, got %d", len(sel.Eligible))
	}
}

func TestResolve_ExplicitIDUsedAndIdempotencyForwarded(t *testing.T) {
	gw := &fakeGateway{liber: phcontracts.Liberacao{Liberado: true}}
	repo := newFakeProfileRepo()
	repo.byID["a"] = &phentity.ISPProfile{ID: "a", TenantID: "t1", Label: "A", ISPType: phentity.ISPIXCSoft, Enabled: true, Transports: []string{phentity.TransportHTTP}, Credentials: map[string]string{"ixcsoft_host": "h"}}
	repo.byID["b"] = &phentity.ISPProfile{ID: "b", TenantID: "t1", Label: "B", ISPType: phentity.ISPMKAuth, IsDefault: true, Enabled: true, Transports: []string{phentity.TransportHTTP}, Credentials: map[string]string{"mkauth_host": "h"}}
	svc := NewQueryService(repo, &fakeLogs{}, &fakeConvRepo{items: map[string]*conventity.Conversation{
		"cv1": {ID: "cv1", TenantID: "t1", ContactID: "c1", SectorID: "s1"},
	}}, &fakeContactRepo{byID: map[string]*contactentity.Contact{"c1": {ID: "c1", TenantID: "t1"}}},
		gw, fakeLimiter{allow: true}, fixedClock{t: time.Unix(1700000000, 0).UTC()})
	svc.SetEnvDefault("http://api", "k")

	// Explicit profile "a" overrides the default "b".
	if _, err := svc.LiberarAcesso(actorCtx("t1", "u1", authz.IntegrationExecuteAction), "cv1", "a", "idc1", "idem-xyz"); err != nil {
		t.Fatalf("liberar: %v", err)
	}
	if gw.gotConfig == nil || gw.gotConfig.ISPType != phentity.ISPIXCSoft {
		t.Errorf("explicit profile not used: %+v", gw.gotConfig)
	}
	if gw.gotIdemKey != "idem-xyz" {
		t.Errorf("idempotency key not forwarded to gateway: %q", gw.gotIdemKey)
	}
}

func TestConsultarCliente_RequireTenant(t *testing.T) {
	svc, _, _ := newSvc(&fakeGateway{}, true, true)
	if _, err := svc.ConsultarCliente(context.Background(), "cv1", phcontracts.ConsultaClienteRequest{}); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expected forbidden without tenant, got %v", err)
	}
}
