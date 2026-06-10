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

// friendlyUnavailable is the user-safe message for any external failure — the
// screen must not break ("falha externa retorna erro amigável").
const friendlyUnavailable = "the provider service is temporarily unavailable, please try again"

// QueryService performs on-demand provider queries scoped to a conversation,
// enforcing tenant + visibility, rate limiting, and minimal logging.
type QueryService struct {
	config        phrepo.ConfigRepository
	logs          phrepo.QueryLogRepository
	conversations convrepo.ConversationRepository
	contacts      contactrepo.ContactRepository
	gateway       phcontracts.Gateway
	limiter       phcontracts.RateLimiter
	clock         shared.Clock
}

// NewQueryService builds the service.
func NewQueryService(
	config phrepo.ConfigRepository,
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
	return &QueryService{config: config, logs: logs, conversations: conversations, contacts: contacts, gateway: gateway, limiter: limiter, clock: clock}
}

// CustomerProfile fetches the customer profile for a conversation's contact.
func (s *QueryService) CustomerProfile(ctx context.Context, conversationID string) (phcontracts.CustomerProfile, error) {
	conv, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return phcontracts.CustomerProfile{}, err
	}
	return runQuery(s, ctx, conv, phentity.QueryCustomerProfile,
		func(cfg *phentity.ProviderIntegrationConfig, lk phcontracts.Lookup) (phcontracts.CustomerProfile, error) {
			return s.gateway.GetCustomerProfile(ctx, cfg, lk)
		})
}

// Contracts fetches the contracts for a conversation's contact.
func (s *QueryService) Contracts(ctx context.Context, conversationID string) ([]phcontracts.Contract, error) {
	conv, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	return runQuery(s, ctx, conv, phentity.QueryContracts,
		func(cfg *phentity.ProviderIntegrationConfig, lk phcontracts.Lookup) ([]phcontracts.Contract, error) {
			return s.gateway.GetContracts(ctx, cfg, lk)
		})
}

// FinancialStatus fetches the financial snapshot (gated by contact.view_financial
// at the route).
func (s *QueryService) FinancialStatus(ctx context.Context, conversationID string) (phcontracts.FinancialStatus, error) {
	conv, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return phcontracts.FinancialStatus{}, err
	}
	return runQuery(s, ctx, conv, phentity.QueryFinancialStatus,
		func(cfg *phentity.ProviderIntegrationConfig, lk phcontracts.Lookup) (phcontracts.FinancialStatus, error) {
			return s.gateway.GetFinancialStatus(ctx, cfg, lk)
		})
}

// ConnectionStatus fetches the connection status (gated by
// contact.view_connection_status at the route).
func (s *QueryService) ConnectionStatus(ctx context.Context, conversationID string) (phcontracts.ConnectionStatus, error) {
	conv, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return phcontracts.ConnectionStatus{}, err
	}
	return runQuery(s, ctx, conv, phentity.QueryConnectionStatus,
		func(cfg *phentity.ProviderIntegrationConfig, lk phcontracts.Lookup) (phcontracts.ConnectionStatus, error) {
			return s.gateway.GetConnectionStatus(ctx, cfg, lk)
		})
}

// Tickets fetches the customer's tickets.
func (s *QueryService) Tickets(ctx context.Context, conversationID string) ([]phcontracts.Ticket, error) {
	conv, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	return runQuery(s, ctx, conv, phentity.QueryTickets,
		func(cfg *phentity.ProviderIntegrationConfig, lk phcontracts.Lookup) ([]phcontracts.Ticket, error) {
			return s.gateway.GetTickets(ctx, cfg, lk)
		})
}

// OpenTicket opens a ticket (gated by integration.execute_action at the route).
func (s *QueryService) OpenTicket(ctx context.Context, conversationID string, input phcontracts.OpenTicketInput) (phcontracts.Ticket, error) {
	conv, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return phcontracts.Ticket{}, err
	}
	return runQuery(s, ctx, conv, phentity.QueryOpenTicket,
		func(cfg *phentity.ProviderIntegrationConfig, lk phcontracts.Lookup) (phcontracts.Ticket, error) {
			return s.gateway.OpenTicket(ctx, cfg, lk, input)
		})
}

// runQuery wraps a gateway call with rate limiting, config resolution, latency
// measurement, minimal logging and friendly error mapping.
func runQuery[T any](
	s *QueryService,
	ctx context.Context,
	conv *conventity.Conversation,
	qtype phentity.QueryType,
	fn func(cfg *phentity.ProviderIntegrationConfig, lookup phcontracts.Lookup) (T, error),
) (T, error) {
	var zero T
	tenantID := conv.TenantID

	if s.limiter != nil {
		if allowed, err := s.limiter.Allow(ctx, tenantID); err == nil && !allowed {
			s.log(ctx, conv, qtype, phentity.StatusBlocked, 0, "rate limited")
			return zero, apperror.RateLimited("too many provider queries, please wait a moment")
		}
	}

	cfg, err := s.config.FindEnabled(ctx)
	if err != nil {
		s.log(ctx, conv, qtype, phentity.StatusError, 0, "no provider config")
		return zero, apperror.Integration("provider integration is not configured")
	}

	lookup := s.lookup(ctx, conv)
	start := s.clock.Now()
	res, callErr := fn(cfg, lookup)
	latency := s.clock.Now().Sub(start).Milliseconds()

	if callErr != nil {
		status := phentity.StatusError
		if isTimeout(callErr) {
			status = phentity.StatusTimeout
		}
		s.log(ctx, conv, qtype, status, latency, summarize(callErr))
		return zero, apperror.Integration(friendlyUnavailable)
	}

	s.log(ctx, conv, qtype, phentity.StatusSuccess, latency, "")
	return res, nil
}

// lookup builds the provider lookup key from the conversation's contact.
func (s *QueryService) lookup(ctx context.Context, conv *conventity.Conversation) phcontracts.Lookup {
	lk := phcontracts.Lookup{ContactID: conv.ContactID}
	contact, err := s.contacts.FindByID(ctx, conv.ContactID)
	if err != nil {
		return lk
	}
	lk.Document = contact.Document
	lk.Phone = contact.Phone
	for _, id := range contact.Identities {
		if id.Channel == conv.Channel {
			lk.ExternalID = id.ExternalID
			break
		}
	}
	return lk
}

// loadVisible loads a conversation and enforces the actor's visibility.
func (s *QueryService) loadVisible(ctx context.Context, id string) (*conventity.Conversation, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return nil, apperror.Unauthorized("authentication required")
	}
	conv, err := s.conversations.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ac.SectorScope == authz.ScopeAll {
		return conv, nil
	}
	if conv.AssignedTo != "" && conv.AssignedTo == ac.UserID {
		return conv, nil
	}
	for _, sid := range ac.SectorIDs {
		if sid == conv.SectorID && sid != "" {
			return conv, nil
		}
	}
	return nil, apperror.NotFound("conversation not found")
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
