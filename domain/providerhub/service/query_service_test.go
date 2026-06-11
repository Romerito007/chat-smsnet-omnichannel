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
	cliente phcontracts.ClienteResult
	planos  []phcontracts.Plano
	empresa phcontracts.Empresa
	liber   phcontracts.Liberacao
	chamado phcontracts.Chamado
	err     error
	gotReq  phcontracts.ConsultaClienteRequest
}

func (g *fakeGateway) ConsultarCliente(_ context.Context, _ *phentity.ProviderIntegrationConfig, req phcontracts.ConsultaClienteRequest) (phcontracts.ClienteResult, error) {
	g.gotReq = req
	return g.cliente, g.err
}
func (g *fakeGateway) ListarPlanos(context.Context, *phentity.ProviderIntegrationConfig) ([]phcontracts.Plano, error) {
	return g.planos, g.err
}
func (g *fakeGateway) DadosEmpresa(context.Context, *phentity.ProviderIntegrationConfig) (phcontracts.Empresa, error) {
	return g.empresa, g.err
}
func (g *fakeGateway) LiberarAcesso(context.Context, *phentity.ProviderIntegrationConfig, string) (phcontracts.Liberacao, error) {
	return g.liber, g.err
}
func (g *fakeGateway) AbrirChamado(context.Context, *phentity.ProviderIntegrationConfig, string, string, string) (phcontracts.Chamado, error) {
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

func newSvc(gw *fakeGateway, limiterAllow, withConfig bool) (*QueryService, *fakeLogs, *fakeAuditor) {
	var cfg *phentity.ProviderIntegrationConfig
	if withConfig {
		cfg = &phentity.ProviderIntegrationConfig{ID: "cfg1", TenantID: "t1", Enabled: true, ISPType: phentity.ISPHubsoft, SMSNetBaseURL: "http://api", TimeoutMs: 1000}
	}
	logs := &fakeLogs{}
	aud := &fakeAuditor{}
	svc := NewQueryService(
		&fakeConfigRepo{cfg: cfg},
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
	res, err := svc.LiberarAcesso(actorCtx("t1", "u1", authz.IntegrationExecuteAction), "cv1", "idc1")
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
	res, err := svc.AbrirChamado(actorCtx("t1", "u1", authz.IntegrationExecuteAction), "cv1", "idc1", "sem internet", "detalhe")
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
	_, err := svc.LiberarAcesso(actorCtx("t1", "u1", authz.IntegrationExecuteAction), "cv1", "  ")
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation for empty id_cliente, got %v", err)
	}
}

func TestExecute_RateLimited(t *testing.T) {
	svc, logs, _ := newSvc(&fakeGateway{}, false, true)
	_, err := svc.ListarPlanos(actorCtx("t1", "u1", authz.IntegrationRead), "cv1")
	if apperror.From(err).Code != apperror.CodeRateLimited {
		t.Fatalf("expected rate_limited, got %v", err)
	}
	if logs.last().Status != phentity.StatusBlocked {
		t.Errorf("log status = %q, want blocked", logs.last().Status)
	}
}

func TestExecute_NoConfig(t *testing.T) {
	svc, _, _ := newSvc(&fakeGateway{}, true, false)
	_, err := svc.DadosEmpresa(actorCtx("t1", "u1", authz.IntegrationRead), "cv1")
	if apperror.From(err).Code != apperror.CodeIntegrationUnavailable {
		t.Errorf("expected integration error without config, got %v", err)
	}
}

func TestConsultarCliente_RequireTenant(t *testing.T) {
	svc, _, _ := newSvc(&fakeGateway{}, true, true)
	if _, err := svc.ConsultarCliente(context.Background(), "cv1", phcontracts.ConsultaClienteRequest{}); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expected forbidden without tenant, got %v", err)
	}
}
