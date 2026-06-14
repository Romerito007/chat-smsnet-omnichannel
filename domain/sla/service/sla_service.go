package service

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sla/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sla/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sla/repository"
)

// checkBatchLimit bounds how many running trackings the scheduler evaluates per
// tick.
const checkBatchLimit = 1000

// Service measures conversations against SLA policies: it starts tracking on
// creation, records first-response/resolution, lists at-risk trackings and runs
// the scheduler breach check. It implements the conversations SLAHook.
type Service struct {
	policies      repository.PolicyRepository
	tracking      repository.TrackingRepository
	conversations convrepo.ConversationRepository
	bizClock      shared.BusinessClock
	publisher     shared.EventPublisher
	webhooks      shared.WebhookEmitter
	notifier      shared.Notifier
	clock         shared.Clock
}

// SetNotifier wires the user notifier. Optional: when unset, the assigned agent
// is not notified of SLA warnings/breaches.
func (s *Service) SetNotifier(n shared.Notifier) {
	if n != nil {
		s.notifier = n
	}
}

// NewService builds the service.
func NewService(
	policies repository.PolicyRepository,
	tracking repository.TrackingRepository,
	conversations convrepo.ConversationRepository,
	bizClock shared.BusinessClock,
	publisher shared.EventPublisher,
	webhooks shared.WebhookEmitter,
	clock shared.Clock,
) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if publisher == nil {
		publisher = shared.NoopPublisher{}
	}
	if webhooks == nil {
		webhooks = shared.NoopWebhookEmitter{}
	}
	if bizClock == nil {
		bizClock = shared.NoopBusinessClock{}
	}
	return &Service{
		policies: policies, tracking: tracking, conversations: conversations,
		bizClock: bizClock, publisher: publisher, webhooks: webhooks,
		notifier: shared.NoopNotifier{}, clock: clock,
	}
}

// ── conversations SLAHook ────────────────────────────────────────────────────

// OnConversationCreated selects the most specific enabled policy and starts a
// tracking with due/warn instants (in business time when required).
func (s *Service) OnConversationCreated(ctx context.Context, conv *conventity.Conversation) {
	if existing, _ := s.tracking.FindByConversation(ctx, conv.ID); existing != nil {
		return // already tracked
	}
	policies, err := s.policies.ListEnabled(ctx)
	if err != nil {
		return
	}
	policy := selectPolicy(policies, conv)
	if policy == nil {
		return
	}

	start := conv.CreatedAt
	if start.IsZero() {
		start = s.clock.Now()
	}
	bizOnly := policy.BusinessHoursOnly
	warnPct := policy.WarningThresholdPct

	t := &entity.SLATracking{
		ID:                     shared.NewID(),
		TenantID:               conv.TenantID,
		ConversationID:         conv.ID,
		PolicyID:               policy.ID,
		SectorID:               conv.SectorID,
		FirstResponseDueAt:     s.computeDue(ctx, conv.ChannelID, start, policy.FirstResponseTargetSec, bizOnly),
		FirstResponseWarnAt:    s.computeDue(ctx, conv.ChannelID, start, pct(policy.FirstResponseTargetSec, warnPct), bizOnly),
		ResolutionDueAt:        s.computeDue(ctx, conv.ChannelID, start, policy.ResolutionTargetSec, bizOnly),
		ResolutionWarnAt:       s.computeDue(ctx, conv.ChannelID, start, pct(policy.ResolutionTargetSec, warnPct), bizOnly),
		PauseOnWaitingCustomer: policy.PauseOnWaitingCustomer,
		Status:                 entity.StatusRunning,
		CreatedAt:              s.clock.Now(),
		UpdatedAt:              s.clock.Now(),
	}
	_ = s.tracking.Create(ctx, t)
}

// OnFirstResponse records the first response time (idempotent) and flags a breach
// if it arrived after the due instant.
func (s *Service) OnFirstResponse(ctx context.Context, conv *conventity.Conversation, at time.Time) {
	t, err := s.tracking.FindByConversation(ctx, conv.ID)
	if err != nil || t == nil || t.FirstResponseAt != nil {
		return
	}
	t.FirstResponseAt = &at
	if t.FirstResponseDueAt != nil && at.After(*t.FirstResponseDueAt) {
		t.FirstResponseBreached = true
	}
	t.UpdatedAt = s.clock.Now()
	_ = s.tracking.Update(ctx, t)
}

// OnResolved records the resolution time, flags a resolution breach and
// finalizes the tracking status.
func (s *Service) OnResolved(ctx context.Context, conv *conventity.Conversation, at time.Time) {
	t, err := s.tracking.FindByConversation(ctx, conv.ID)
	if err != nil || t == nil || t.ResolvedAt != nil {
		return
	}
	t.ResolvedAt = &at
	if t.ResolutionDueAt != nil && at.After(*t.ResolutionDueAt) {
		t.ResolutionBreached = true
	}
	// A conversation resolved without ever responding, past first-response due, is
	// a first-response breach too.
	if t.FirstResponseAt == nil && t.FirstResponseDueAt != nil && at.After(*t.FirstResponseDueAt) {
		t.FirstResponseBreached = true
	}
	t.Finalize()
	t.UpdatedAt = s.clock.Now()
	_ = s.tracking.Update(ctx, t)
}

// ── queries ──────────────────────────────────────────────────────────────────

// Status returns the SLA tracking for a conversation.
func (s *Service) Status(ctx context.Context, conversationID string) (*entity.SLATracking, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.tracking.FindByConversation(ctx, conversationID)
}

// AtRisk lists the tenant's running trackings that have warned or breached.
func (s *Service) AtRisk(ctx context.Context, page shared.PageRequest) ([]*entity.SLATracking, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.tracking.ListAtRisk(ctx, page.Normalize())
}

// ── scheduler check ──────────────────────────────────────────────────────────

// RunCheck evaluates running trackings across all tenants, firing warnings at
// the threshold and breaches past due. It is invoked by the sla.check job.
func (s *Service) RunCheck(ctx context.Context) error {
	items, err := s.tracking.ListRunningAcrossTenants(ctx, checkBatchLimit)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	// Group trackings by tenant and hydrate each tenant's conversations in ONE
	// batch ($in), instead of a FindByID per tracking.
	byTenant := make(map[string][]*entity.SLATracking)
	for _, t := range items {
		byTenant[t.TenantID] = append(byTenant[t.TenantID], t)
	}
	for tenantID, trackings := range byTenant {
		tctx := authz.WithAuthContext(shared.WithTenant(ctx, tenantID), authz.SystemActor(tenantID))
		ids := make([]string, 0, len(trackings))
		for _, t := range trackings {
			ids = append(ids, t.ConversationID)
		}
		convs, err := s.conversations.FindByIDs(tctx, ids)
		if err != nil {
			continue // best-effort per tenant: a hiccup must not block the others
		}
		byID := make(map[string]*conventity.Conversation, len(convs))
		for _, c := range convs {
			byID[c.ID] = c
		}
		for _, t := range trackings {
			if conv, ok := byID[t.ConversationID]; ok {
				s.evaluate(tctx, t, conv, now)
			}
		}
	}
	return nil
}

// evaluate measures one tracking against its (already-loaded) conversation.
func (s *Service) evaluate(tctx context.Context, t *entity.SLATracking, conv *conventity.Conversation, now time.Time) {
	// Pause: suppress alerts while waiting on the customer when configured.
	if t.PauseOnWaitingCustomer && conv.Status == conventity.StatusWaitingCustomer {
		return
	}

	changed := false
	// First-response leg.
	if t.FirstResponseAt == nil && t.FirstResponseDueAt != nil {
		if !t.FirstResponseBreached && !now.Before(*t.FirstResponseDueAt) {
			t.FirstResponseBreached, t.FirstResponseWarned, changed = true, true, true
			s.fire(tctx, t, conv, "first_response", contracts.RealtimeSLABreached, true)
		} else if !t.FirstResponseWarned && t.FirstResponseWarnAt != nil && !now.Before(*t.FirstResponseWarnAt) {
			t.FirstResponseWarned, changed = true, true
			s.fire(tctx, t, conv, "first_response", contracts.RealtimeSLAWarning, false)
		}
	}
	// Resolution leg.
	if t.ResolvedAt == nil && t.ResolutionDueAt != nil {
		if !t.ResolutionBreached && !now.Before(*t.ResolutionDueAt) {
			t.ResolutionBreached, t.ResolutionWarned, changed = true, true, true
			s.fire(tctx, t, conv, "resolution", contracts.RealtimeSLABreached, true)
		} else if !t.ResolutionWarned && t.ResolutionWarnAt != nil && !now.Before(*t.ResolutionWarnAt) {
			t.ResolutionWarned, changed = true, true
			s.fire(tctx, t, conv, "resolution", contracts.RealtimeSLAWarning, false)
		}
	}
	if changed {
		t.UpdatedAt = now
		_ = s.tracking.Update(tctx, t)
	}
}

// fire publishes the realtime event and, for breaches, emits the sla.breached
// webhook.
func (s *Service) fire(ctx context.Context, t *entity.SLATracking, conv *conventity.Conversation, leg, event string, breach bool) {
	payload := map[string]any{
		"conversation_id":       t.ConversationID,
		"policy_id":             t.PolicyID,
		"leg":                   leg,
		"sector_id":             conv.SectorID,
		"first_response_due_at": t.FirstResponseDueAt,
		"resolution_due_at":     t.ResolutionDueAt,
		"breached":              breach,
	}
	_ = s.publisher.Publish(ctx, shared.TopicConversation(t.TenantID, t.ConversationID), event, payload)
	_ = s.publisher.Publish(ctx, shared.TopicTenant(t.TenantID), event, payload)
	if breach {
		s.webhooks.Emit(ctx, t.TenantID, contracts.RealtimeSLABreached, conv.SectorID, payload)
	}
	// Notify the assigned agent (in-app + maybe email per their preference).
	if conv.AssignedTo != "" {
		ntype, title := "sla.at_risk", "A conversation is at risk of breaching SLA"
		if breach {
			ntype, title = "sla.breached", "A conversation breached its SLA"
		}
		s.notifier.Notify(ctx, shared.NotifyInput{
			TenantID: t.TenantID, UserID: conv.AssignedTo,
			Type: ntype, Title: title, Link: "/conversations/" + t.ConversationID,
		})
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// selectPolicy returns the most specific enabled policy matching the
// conversation, or nil.
func selectPolicy(policies []*entity.SLAPolicy, conv *conventity.Conversation) *entity.SLAPolicy {
	var best *entity.SLAPolicy
	for _, p := range policies {
		if !p.Enabled || !p.Matches(conv.SectorID, string(conv.Priority), conv.Channel) {
			continue
		}
		if best == nil || p.Specificity() > best.Specificity() {
			best = p
		}
	}
	return best
}

func (s *Service) computeDue(ctx context.Context, channelID string, from time.Time, targetSec int, bizOnly bool) *time.Time {
	if targetSec <= 0 {
		return nil
	}
	d := time.Duration(targetSec) * time.Second
	var due time.Time
	if bizOnly {
		// Resolve in the channel's business hours. An empty/unknown channel id (e.g.
		// a manually-created conversation) makes AddBusinessDuration error → fall
		// back to wall-clock.
		t, err := s.bizClock.AddBusinessDuration(ctx, channelID, from, d)
		if err != nil {
			t = from.Add(d)
		}
		due = t
	} else {
		due = from.Add(d)
	}
	return &due
}

// pct returns targetSec * percent / 100 (the warning offset).
func pct(targetSec, percent int) int {
	if targetSec <= 0 {
		return 0
	}
	return targetSec * percent / 100
}

var _ convcontracts.SLAHook = (*Service)(nil)
