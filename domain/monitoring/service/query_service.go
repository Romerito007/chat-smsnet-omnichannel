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
	mcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/contracts"
	mentity "github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/entity"
	mrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// friendlyUnavailable is the user-safe message for any external failure — the
// screen must not break.
const friendlyUnavailable = "the monitoring service is temporarily unavailable, please try again"

// QueryService performs on-demand monitoring queries scoped to a conversation,
// enforcing tenant + visibility, rate limiting, and minimal logging.
type QueryService struct {
	config        mrepo.ConfigRepository
	logs          mrepo.QueryLogRepository
	conversations convrepo.ConversationRepository
	contacts      contactrepo.ContactRepository
	gateway       mcontracts.Gateway
	limiter       mcontracts.RateLimiter
	clock         shared.Clock
}

// NewQueryService builds the service.
func NewQueryService(
	config mrepo.ConfigRepository,
	logs mrepo.QueryLogRepository,
	conversations convrepo.ConversationRepository,
	contacts contactrepo.ContactRepository,
	gateway mcontracts.Gateway,
	limiter mcontracts.RateLimiter,
	clock shared.Clock,
) *QueryService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &QueryService{config: config, logs: logs, conversations: conversations, contacts: contacts, gateway: gateway, limiter: limiter, clock: clock}
}

// Summary fetches the normalized monitoring summary for a conversation's contact.
func (s *QueryService) Summary(ctx context.Context, conversationID string) (mcontracts.MonitoringSummary, error) {
	conv, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return mcontracts.MonitoringSummary{}, err
	}
	return runQuery(s, ctx, conv, mentity.QuerySummary,
		func(cfg *mentity.MonitoringIntegrationConfig, lk mcontracts.Lookup) (mcontracts.MonitoringSummary, error) {
			return s.gateway.GetSummary(ctx, cfg, lk)
		})
}

// Incidents fetches the customer's monitoring incidents.
func (s *QueryService) Incidents(ctx context.Context, conversationID string) ([]mcontracts.Incident, error) {
	conv, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	return runQuery(s, ctx, conv, mentity.QueryIncidents,
		func(cfg *mentity.MonitoringIntegrationConfig, lk mcontracts.Lookup) ([]mcontracts.Incident, error) {
			return s.gateway.GetIncidents(ctx, cfg, lk)
		})
}

// runQuery wraps a gateway call with rate limiting, config resolution, latency
// measurement, minimal logging and friendly error mapping.
func runQuery[T any](
	s *QueryService,
	ctx context.Context,
	conv *conventity.Conversation,
	qtype mentity.QueryType,
	fn func(cfg *mentity.MonitoringIntegrationConfig, lookup mcontracts.Lookup) (T, error),
) (T, error) {
	var zero T
	tenantID := conv.TenantID

	if s.limiter != nil {
		if allowed, err := s.limiter.Allow(ctx, tenantID); err == nil && !allowed {
			s.log(ctx, conv, qtype, mentity.StatusBlocked, 0, "rate limited")
			return zero, apperror.RateLimited("too many monitoring queries, please wait a moment")
		}
	}

	cfg, err := s.config.FindEnabled(ctx)
	if err != nil {
		s.log(ctx, conv, qtype, mentity.StatusError, 0, "no monitoring config")
		return zero, apperror.Integration("monitoring integration is not configured")
	}

	lookup := s.lookup(ctx, conv)
	start := s.clock.Now()
	res, callErr := fn(cfg, lookup)
	latency := s.clock.Now().Sub(start).Milliseconds()

	if callErr != nil {
		status := mentity.StatusError
		if isTimeout(callErr) {
			status = mentity.StatusTimeout
		}
		s.log(ctx, conv, qtype, status, latency, summarize(callErr))
		return zero, apperror.Integration(friendlyUnavailable)
	}

	s.log(ctx, conv, qtype, mentity.StatusSuccess, latency, "")
	return res, nil
}

// lookup builds the monitoring lookup key from the conversation's contact.
func (s *QueryService) lookup(ctx context.Context, conv *conventity.Conversation) mcontracts.Lookup {
	lk := mcontracts.Lookup{ContactID: conv.ContactID}
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

// loadVisible loads a conversation and enforces the actor's tenant + visibility.
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

func (s *QueryService) log(ctx context.Context, conv *conventity.Conversation, qtype mentity.QueryType, status mentity.QueryStatus, latencyMs int64, summary string) {
	userID := ""
	if ac, ok := authz.FromContext(ctx); ok {
		userID = ac.UserID
	}
	_ = s.logs.Create(ctx, &mentity.MonitoringQueryLog{
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
