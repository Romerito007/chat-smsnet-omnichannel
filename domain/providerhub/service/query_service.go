package service

import (
	"context"
	"errors"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/repository"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	phcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/contracts"
	phentity "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	phrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// QueryService performs on-demand smsnet-integrations queries scoped to a
// conversation, enforcing tenant + visibility, rate limiting and minimal logging.
// The ISP profile is resolved per call (explicit isp_config_id > tenant default);
// the gateway call config is built at the edge from the infra gateway (env
// host/key) + the resolved profile.
type QueryService struct {
	resolver      *ISPResolver
	logs          phrepo.QueryLogRepository
	conversations convrepo.ConversationRepository
	contacts      contactrepo.ContactRepository
	gateway       phcontracts.Gateway
	limiter       phcontracts.RateLimiter
	clock         shared.Clock
	auditor       shared.Auditor
	envHost       string
	envKey        string
}

// SetEnvDefault wires the infra SMSNET gateway host/key (ISP_GATEWAY_API_HOST/KEY).
func (s *QueryService) SetEnvDefault(host, key string) {
	s.envHost, s.envKey = host, key
}

// NewQueryService builds the service.
func NewQueryService(
	profiles phrepo.ProfileRepository,
	logs phrepo.QueryLogRepository,
	conversations convrepo.ConversationRepository,
	contacts contactrepo.ContactRepository,
	gateway phcontracts.Gateway,
	limiter phcontracts.RateLimiter,
	clock shared.Clock,
) *QueryService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &QueryService{resolver: NewISPResolver(profiles), logs: logs, conversations: conversations, contacts: contacts, gateway: gateway, limiter: limiter, clock: clock, auditor: shared.NoopAuditor{}}
}

// resolveCall resolves the ISP profile for this call and builds the effective
// gateway config. Returns an ISPSelectionRequiredError when the profile is
// ambiguous, or a clear error when no profile / no gateway is configured.
func (s *QueryService) resolveCall(ctx context.Context, conv *conventity.Conversation, explicitID string) (*phentity.ProviderIntegrationConfig, error) {
	if s.envHost == "" {
		return nil, apperror.Integration("provider integration is not configured")
	}
	rr, err := s.resolver.Resolve(ctx, explicitID)
	if err != nil {
		return nil, err
	}
	switch rr.Status {
	case ResolveOK:
		return buildCallConfig(conv.TenantID, s.envHost, s.envKey, rr.Profile), nil
	case ResolveAmbiguous:
		return nil, &ISPSelectionRequiredError{Eligible: rr.Eligible}
	default: // ResolveNone
		return nil, apperror.Conflict("nenhum perfil de ISP configurado para o tenant")
	}
}

// SetAuditor wires the audit trail. Optional: when unset, side-effect actions are
// not audited.
func (s *QueryService) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// ConsultarCliente looks up the customer. When the request omits all identifiers
// it falls back to the conversation contact's document/phone. A needs_input
// response returns ClienteResult{NeedsSelection:true, Options:[...]} for the agent
// to pick a contract; the next call carries IDCliente. Faturas are omitted unless
// the actor holds contact.view_financial.
func (s *QueryService) ConsultarCliente(ctx context.Context, conversationID string, req phcontracts.ConsultaClienteRequest) (phcontracts.ClienteResult, error) {
	conv, ac, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return phcontracts.ClienteResult{}, err
	}
	req = s.fillFromContact(ctx, conv, req)

	cfg, err := s.resolveCall(ctx, conv, req.ISPConfigID)
	if err != nil {
		return phcontracts.ClienteResult{}, err
	}
	res, err := execute(s, ctx, conv, phentity.QueryConsultarCliente, cfg,
		func(cfg *phentity.ProviderIntegrationConfig) (phcontracts.ClienteResult, error) {
			return s.gateway.ConsultarCliente(ctx, cfg, req)
		})
	if err != nil {
		return phcontracts.ClienteResult{}, err
	}
	// Omit the invoices for agents without the financial permission.
	if res.Cliente != nil && !ac.Has(authz.ContactViewFinancial) {
		res.Cliente.Faturas = nil
	}
	return res, nil
}

// ListarPlanos returns the tenant's plans/offers.
func (s *QueryService) ListarPlanos(ctx context.Context, conversationID, ispConfigID string) ([]phcontracts.Plano, error) {
	conv, _, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	cfg, err := s.resolveCall(ctx, conv, ispConfigID)
	if err != nil {
		return nil, err
	}
	return execute(s, ctx, conv, phentity.QueryListarPlanos, cfg,
		func(cfg *phentity.ProviderIntegrationConfig) ([]phcontracts.Plano, error) {
			return s.gateway.ListarPlanos(ctx, cfg)
		})
}

// DadosEmpresa returns the company/ISP profile.
func (s *QueryService) DadosEmpresa(ctx context.Context, conversationID, ispConfigID string) (phcontracts.Empresa, error) {
	conv, _, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return phcontracts.Empresa{}, err
	}
	cfg, err := s.resolveCall(ctx, conv, ispConfigID)
	if err != nil {
		return phcontracts.Empresa{}, err
	}
	return execute(s, ctx, conv, phentity.QueryDadosEmpresa, cfg,
		func(cfg *phentity.ProviderIntegrationConfig) (phcontracts.Empresa, error) {
			return s.gateway.DadosEmpresa(ctx, cfg)
		})
}

// LiberarAcesso performs a trust-unlock for a customer contract (side effect).
// Audited as providerhub.liberacao. The idempotencyKey is forwarded to the gateway
// so the upstream API can dedup retries.
func (s *QueryService) LiberarAcesso(ctx context.Context, conversationID, ispConfigID, idCliente, idempotencyKey string) (phcontracts.Liberacao, error) {
	conv, _, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return phcontracts.Liberacao{}, err
	}
	if strings.TrimSpace(idCliente) == "" {
		return phcontracts.Liberacao{}, apperror.Validation("id_cliente is required")
	}
	cfg, err := s.resolveCall(ctx, conv, ispConfigID)
	if err != nil {
		return phcontracts.Liberacao{}, err
	}
	ctx = phcontracts.WithIdempotencyKey(ctx, idempotencyKey)
	res, err := execute(s, ctx, conv, phentity.QueryLiberarAcesso, cfg,
		func(cfg *phentity.ProviderIntegrationConfig) (phcontracts.Liberacao, error) {
			return s.gateway.LiberarAcesso(ctx, cfg, idCliente)
		})
	if err != nil {
		return phcontracts.Liberacao{}, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "providerhub.liberacao", ResourceType: "conversation", ResourceID: conv.ID,
		Data: map[string]any{"id_cliente": idCliente, "liberado": res.Liberado, "protocolo": res.Protocolo},
	})
	return res, nil
}

// AbrirChamado opens a support ticket for a customer contract (side effect).
// Audited as providerhub.chamado. The idempotencyKey is forwarded to the gateway.
func (s *QueryService) AbrirChamado(ctx context.Context, conversationID, ispConfigID, idCliente, subject, message, idempotencyKey string) (phcontracts.Chamado, error) {
	conv, _, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return phcontracts.Chamado{}, err
	}
	if strings.TrimSpace(idCliente) == "" {
		return phcontracts.Chamado{}, apperror.Validation("id_cliente is required")
	}
	if strings.TrimSpace(subject) == "" {
		return phcontracts.Chamado{}, apperror.Validation("subject is required")
	}
	cfg, err := s.resolveCall(ctx, conv, ispConfigID)
	if err != nil {
		return phcontracts.Chamado{}, err
	}
	ctx = phcontracts.WithIdempotencyKey(ctx, idempotencyKey)
	res, err := execute(s, ctx, conv, phentity.QueryAbrirChamado, cfg,
		func(cfg *phentity.ProviderIntegrationConfig) (phcontracts.Chamado, error) {
			return s.gateway.AbrirChamado(ctx, cfg, idCliente, subject, message)
		})
	if err != nil {
		return phcontracts.Chamado{}, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "providerhub.chamado", ResourceType: "conversation", ResourceID: conv.ID,
		Data: map[string]any{"id_cliente": idCliente, "subject": subject, "protocolo": res.Protocolo},
	})
	return res, nil
}

// execute wraps a resolved gateway call with rate limiting, latency measurement
// and minimal logging. The ISP profile is already resolved into cfg by the caller.
// The gateway returns domain errors (not_found → friendly NotFound, fallback →
// Integration), which pass through.
func execute[T any](
	s *QueryService,
	ctx context.Context,
	conv *conventity.Conversation,
	qtype phentity.QueryType,
	cfg *phentity.ProviderIntegrationConfig,
	fn func(cfg *phentity.ProviderIntegrationConfig) (T, error),
) (T, error) {
	var zero T

	if s.limiter != nil {
		if allowed, err := s.limiter.Allow(ctx, conv.TenantID); err == nil && !allowed {
			s.log(ctx, conv, qtype, phentity.StatusBlocked, 0, "rate limited")
			return zero, apperror.RateLimited("too many provider queries, please wait a moment")
		}
	}

	start := s.clock.Now()
	res, callErr := fn(cfg)
	latency := s.clock.Now().Sub(start).Milliseconds()

	s.log(ctx, conv, qtype, statusFor(callErr), latency, summarize0(callErr))
	return res, callErr
}

// statusFor maps a gateway error to a query-log status.
func statusFor(err error) phentity.QueryStatus {
	switch {
	case err == nil:
		return phentity.StatusSuccess
	case apperror.From(err).Code == apperror.CodeNotFound:
		return phentity.StatusNotFound
	case isTimeout(err):
		return phentity.StatusTimeout
	default:
		return phentity.StatusError
	}
}

func summarize0(err error) string {
	if err == nil {
		return ""
	}
	return summarize(err)
}

// fillFromContact fills missing identifiers from the conversation's contact so the
// agent can open the panel without retyping the customer's document.
func (s *QueryService) fillFromContact(ctx context.Context, conv *conventity.Conversation, req phcontracts.ConsultaClienteRequest) phcontracts.ConsultaClienteRequest {
	if req.CpfCnpj != "" || req.Phone != "" || req.Email != "" || req.IDCliente != "" {
		return req
	}
	contact, err := s.contacts.FindByID(ctx, conv.ContactID)
	if err != nil {
		return req
	}
	req.CpfCnpj = contact.Document
	req.Phone = contact.Phone
	return req
}

// loadVisible loads a conversation and enforces the actor's visibility.
func (s *QueryService) loadVisible(ctx context.Context, id string) (*conventity.Conversation, authz.AuthContext, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, authz.AuthContext{}, err
	}
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return nil, authz.AuthContext{}, apperror.Unauthorized("authentication required")
	}
	conv, err := s.conversations.FindByID(ctx, id)
	if err != nil {
		return nil, ac, err
	}
	if ac.SectorScope == authz.ScopeAll {
		return conv, ac, nil
	}
	if conv.AssignedTo != "" && conv.AssignedTo == ac.UserID {
		return conv, ac, nil
	}
	for _, sid := range ac.SectorIDs {
		if sid == conv.SectorID && sid != "" {
			return conv, ac, nil
		}
	}
	return nil, ac, apperror.NotFound("conversation not found")
}

func (s *QueryService) log(ctx context.Context, conv *conventity.Conversation, qtype phentity.QueryType, status phentity.QueryStatus, latencyMs int64, summary string) {
	userID := ""
	if ac, ok := authz.FromContext(ctx); ok {
		userID = ac.UserID
	}
	_ = s.logs.Create(ctx, &phentity.ProviderQueryLog{
		ID:             shared.NewID(),
		TenantID:       conv.TenantID,
		UserID:         userID,
		ContactID:      conv.ContactID,
		ConversationID: conv.ID,
		QueryType:      qtype,
		Status:         status,
		LatencyMs:      latencyMs,
		ErrorSummary:   summary,
		CreatedAt:      s.clock.Now(),
	})
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}
