// Package service holds the routing business logic: manual and automatic
// (least-loaded) conversation assignment, transfer and enqueue, with a Redis
// lock to prevent double assignment, plus timeline events and realtime fan-out.
package service

import (
	"context"
	"sort"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	presenceentity "github.com/romerito007/chat-smsnet-omnichannel/domain/presence/entity"
	presencerepo "github.com/romerito007/chat-smsnet-omnichannel/domain/presence/repository"
	queuerepo "github.com/romerito007/chat-smsnet-omnichannel/domain/queues/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/routing/contracts"
	sectorrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// lockTTL bounds how long a routing lock is held if a node dies mid-operation.
const lockTTL = 5 * time.Second

// Service implements routing.
type Service struct {
	conversations convrepo.ConversationRepository
	events        convrepo.EventRepository
	presence      presencerepo.PresenceStore
	load          presencerepo.LoadCounter
	users         iamrepo.UserRepository
	sectors       sectorrepo.SectorRepository
	queues        queuerepo.QueueRepository
	locker        shared.Locker
	publisher     shared.EventPublisher
	clock         shared.Clock
	webhooks      shared.WebhookEmitter
	enricher      convcontracts.WebhookEnricher
	notifier      shared.Notifier
	auditor       shared.Auditor
	queueStats    shared.QueueStatsNotifier
}

// SetWebhookEnricher wires the resolver of the outbound-webhook contact + agent
// blocks for assignment/transfer events. Optional and lazy (resolved only when a
// subscription matches the event).
func (s *Service) SetWebhookEnricher(e convcontracts.WebhookEnricher) {
	if e != nil {
		s.enricher = e
	}
}

// emitConversationWebhook emits a conversation webhook with the lazy integration
// payload (custom_attributes + recipient contact + assigned agent), resolved only
// when a subscription matches the event.
func (s *Service) emitConversationWebhook(ctx context.Context, conv *conventity.Conversation, event string) {
	s.webhooks.EmitLazy(ctx, conv.TenantID, event, conv.SectorID, func() any {
		var contact *convcontracts.WebhookContact
		var agent *convcontracts.WebhookAgent
		if s.enricher != nil {
			if conv.ContactID != "" {
				contact = s.enricher.WebhookContact(ctx, conv.ContactID)
			}
			if conv.AssignedTo != "" {
				agent = s.enricher.WebhookAgent(ctx, conv.AssignedTo)
			}
		}
		return convcontracts.NewIntegrationConversationPayload(conv, contact, agent)
	})
}

// SetAuditor wires the audit trail. Optional: when unset, transfers are not
// audited.
func (s *Service) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// SetQueueStatsNotifier wires the queue.stats notifier. Optional: when unset,
// queue-composition changes are not broadcast.
func (s *Service) SetQueueStatsNotifier(n shared.QueueStatsNotifier) {
	if n != nil {
		s.queueStats = n
	}
}

// SetWebhookEmitter wires the outbound webhook emitter. Optional: when unset,
// assignment/transfer events are not forwarded to webhook subscriptions.
func (s *Service) SetWebhookEmitter(e shared.WebhookEmitter) {
	if e != nil {
		s.webhooks = e
	}
}

// SetNotifier wires the user notifier. Optional: when unset, agents are not
// notified of assignments/transfers.
func (s *Service) SetNotifier(n shared.Notifier) {
	if n != nil {
		s.notifier = n
	}
}

// New builds the routing service.
func New(
	conversations convrepo.ConversationRepository,
	events convrepo.EventRepository,
	presence presencerepo.PresenceStore,
	load presencerepo.LoadCounter,
	users iamrepo.UserRepository,
	sectors sectorrepo.SectorRepository,
	queues queuerepo.QueueRepository,
	locker shared.Locker,
	publisher shared.EventPublisher,
	clock shared.Clock,
) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if publisher == nil {
		publisher = shared.NoopPublisher{}
	}
	if locker == nil {
		locker = shared.NoopLocker{}
	}
	return &Service{
		conversations: conversations,
		events:        events,
		presence:      presence,
		load:          load,
		users:         users,
		sectors:       sectors,
		queues:        queues,
		locker:        locker,
		publisher:     publisher,
		clock:         clock,
		webhooks:      shared.NoopWebhookEmitter{},
		notifier:      shared.NoopNotifier{},
		auditor:       shared.NoopAuditor{},
		queueStats:    shared.NoopQueueStatsNotifier{},
	}
}

// candidate is an eligible agent with its current load.
type candidate struct {
	UserID string
	Load   int
}

// Assign manually assigns a conversation to a specific agent.
func (s *Service) Assign(ctx context.Context, conversationID, agentID string) (*conventity.Conversation, error) {
	return s.withLock(ctx, conversationID, func(conv *conventity.Conversation) (*conventity.Conversation, error) {
		if conv.Status.IsClosed() {
			return nil, apperror.Conflict("closed conversation cannot be assigned")
		}
		if conv.SectorID == "" {
			return nil, apperror.Validation("conversation has no sector to assign within")
		}
		// Manual assignment allows an offline/unavailable agent (allowOffline=true);
		// sector membership and capacity still apply.
		if err := s.evaluateAgent(ctx, agentID, conv.SectorID, true); err != nil {
			return nil, err
		}
		return s.applyAssignment(ctx, conv, agentID)
	})
}

// AutoAssign automatically assigns a conversation to the least-loaded eligible
// agent in its sector.
func (s *Service) AutoAssign(ctx context.Context, conversationID string) (*conventity.Conversation, error) {
	return s.withLock(ctx, conversationID, func(conv *conventity.Conversation) (*conventity.Conversation, error) {
		if conv.Status.IsClosed() {
			return nil, apperror.Conflict("closed conversation cannot be assigned")
		}
		if conv.AssignedTo != "" {
			return nil, apperror.Conflict("conversation is already assigned")
		}
		if conv.SectorID == "" {
			return nil, apperror.Validation("conversation has no sector")
		}
		cands, err := s.eligibleAgents(ctx, conv.SectorID)
		if err != nil {
			return nil, err
		}
		if len(cands) == 0 {
			return nil, apperror.Conflict("no eligible agents available")
		}
		return s.applyAssignment(ctx, conv, cands[0].UserID)
	})
}

// Transfer moves a conversation to another sector and/or agent.
func (s *Service) Transfer(ctx context.Context, conversationID string, cmd contracts.TransferCommand) (*conventity.Conversation, error) {
	return s.withLock(ctx, conversationID, func(conv *conventity.Conversation) (*conventity.Conversation, error) {
		if conv.Status.IsClosed() {
			return nil, apperror.Conflict("closed conversation cannot be transferred")
		}
		fromSector := conv.SectorID
		fromAgent := conv.AssignedTo
		fromQueue := conv.QueueID

		if cmd.SectorID != "" && cmd.SectorID != conv.SectorID {
			if _, err := s.sectors.FindByID(ctx, cmd.SectorID); err != nil {
				if apperror.From(err).Code == apperror.CodeNotFound {
					return nil, apperror.Validation("target sector does not exist").
						WithDetails(map[string]any{"sector_id": "not found"})
				}
				return nil, err
			}
			conv.SectorID = cmd.SectorID
		}

		now := s.clock.Now()
		if cmd.AgentID != "" {
			if conv.SectorID == "" {
				return nil, apperror.Validation("transfer to an agent requires a sector")
			}
			// Transfer with a target agent is a manual choice by the operator, so it
			// follows the manual rule: an offline/unavailable agent is allowed (sector
			// membership and capacity still enforced).
			if err := s.evaluateAgent(ctx, cmd.AgentID, conv.SectorID, true); err != nil {
				return nil, err
			}
			conv.AssignedTo = cmd.AgentID
			conv.Status = conventity.StatusAssigned
			conv.QueueID = ""
		} else {
			// Sector-only transfer: unassign and mark transferred for re-routing.
			conv.AssignedTo = ""
			conv.Status = conventity.StatusTransferred
		}
		conv.UpdatedAt = now
		if err := s.conversations.Update(ctx, conv); err != nil {
			return nil, err
		}

		s.recordEvent(ctx, conv, conventity.EventConversationTransferred, map[string]any{
			"from_sector": fromSector,
			"to_sector":   conv.SectorID,
			"from_agent":  fromAgent,
			"to_agent":    conv.AssignedTo,
		})
		s.publishTransferred(ctx, conv, fromSector)
		// A transfer pulls the conversation out of its previous queue.
		if fromQueue != "" {
			s.queueStats.QueueChanged(ctx, fromSector, fromQueue)
		}
		s.emitConversationWebhook(ctx, conv, conventity.EventConversationTransferred)
		_ = s.auditor.Record(ctx, shared.AuditEntry{
			Action: "conversation.transferred", ResourceType: "conversation", ResourceID: conv.ID,
			Data: map[string]any{
				"from_sector": fromSector, "to_sector": conv.SectorID,
				"from_agent": fromAgent, "to_agent": conv.AssignedTo,
			},
		})
		if conv.AssignedTo != "" {
			s.notifier.Notify(ctx, shared.NotifyInput{
				TenantID: conv.TenantID, UserID: conv.AssignedTo,
				Type:  "conversation.transferred_to_you",
				Title: "A conversation was transferred to you",
				Link:  "/conversations/" + conv.ID,
			})
		}
		return conv, nil
	})
}

// Enqueue places a conversation into a queue (sector derived from the queue).
func (s *Service) Enqueue(ctx context.Context, conversationID string, cmd contracts.EnqueueCommand) (*conventity.Conversation, error) {
	return s.withLock(ctx, conversationID, func(conv *conventity.Conversation) (*conventity.Conversation, error) {
		if conv.Status.IsClosed() {
			return nil, apperror.Conflict("closed conversation cannot be enqueued")
		}
		queue, err := s.queues.FindByID(ctx, cmd.QueueID)
		if err != nil {
			if apperror.From(err).Code == apperror.CodeNotFound {
				return nil, apperror.Validation("queue does not exist").
					WithDetails(map[string]any{"queue_id": "not found"})
			}
			return nil, err
		}

		fromQueue, fromSector := conv.QueueID, conv.SectorID
		conv.QueueID = queue.ID
		conv.SectorID = queue.SectorID
		conv.AssignedTo = ""
		conv.Status = conventity.StatusQueued
		conv.UpdatedAt = s.clock.Now()
		if err := s.conversations.Update(ctx, conv); err != nil {
			return nil, err
		}

		s.recordEvent(ctx, conv, conventity.EventConversationEnqueued, map[string]any{
			"queue_id":  queue.ID,
			"sector_id": queue.SectorID,
		})
		s.publishUpdated(ctx, conv)
		// The conversation entered this queue (waiting++); if it was waiting in a
		// different queue before, that one shrank too.
		s.queueStats.QueueChanged(ctx, queue.SectorID, queue.ID)
		if fromQueue != "" && fromQueue != queue.ID {
			s.queueStats.QueueChanged(ctx, fromSector, fromQueue)
		}
		return conv, nil
	})
}

// Run performs automatic routing: a single conversation when ConversationID is
// set, otherwise a batch of waiting (queued/new) conversations.
func (s *Service) Run(ctx context.Context, cmd contracts.RunCommand) (contracts.RunResult, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return contracts.RunResult{}, err
	}

	if cmd.ConversationID != "" {
		conv, err := s.AutoAssign(ctx, cmd.ConversationID)
		if err != nil {
			return contracts.RunResult{}, err
		}
		return contracts.RunResult{Assigned: []contracts.AssignmentResult{
			{ConversationID: conv.ID, AgentID: conv.AssignedTo},
		}}, nil
	}
	return s.runBatch(ctx)
}

// runBatch routes the waiting conversations the actor can see.
func (s *Service) runBatch(ctx context.Context) (contracts.RunResult, error) {
	vis, err := s.visibility(ctx)
	if err != nil {
		return contracts.RunResult{}, err
	}

	result := contracts.RunResult{}
	for _, status := range []conventity.Status{conventity.StatusQueued, conventity.StatusNew} {
		convs, err := s.conversations.List(ctx, convcontracts.ListFilter{Status: string(status)}, vis, shared.PageRequest{Limit: 100})
		if err != nil {
			return contracts.RunResult{}, err
		}
		for _, conv := range convs {
			if conv.SectorID == "" || conv.AssignedTo != "" {
				result.Skipped = append(result.Skipped, contracts.SkippedResult{ConversationID: conv.ID, Reason: "no sector or already assigned"})
				continue
			}
			assigned, err := s.AutoAssign(ctx, conv.ID)
			if err != nil {
				result.Skipped = append(result.Skipped, contracts.SkippedResult{ConversationID: conv.ID, Reason: apperror.From(err).Message})
				continue
			}
			result.Assigned = append(result.Assigned, contracts.AssignmentResult{ConversationID: assigned.ID, AgentID: assigned.AssignedTo})
		}
	}
	return result, nil
}

// ── eligibility & scoring ────────────────────────────────────────────────────

// eligibleAgents returns the agents eligible for a sector, ordered least-loaded
// first (with a deterministic id tiebreaker).
func (s *Service) eligibleAgents(ctx context.Context, sectorID string) ([]candidate, error) {
	users, err := s.users.ListBySector(ctx, sectorID)
	if err != nil {
		return nil, err
	}
	// One aggregation for the whole tenant's loads, instead of a count per agent.
	loads, err := s.load.OpenAssignedLoads(ctx)
	if err != nil {
		return nil, err
	}
	var cands []candidate
	for _, u := range users {
		if !u.IsActive() {
			continue
		}
		p, err := s.presence.Get(ctx, u.ID)
		if err != nil {
			continue // no presence record → offline
		}
		if p.Status != presenceentity.StatusAvailable {
			continue
		}
		load := loads[u.ID]
		if u.MaxConcurrentChats > 0 && load >= u.MaxConcurrentChats {
			continue
		}
		cands = append(cands, candidate{UserID: u.ID, Load: load})
	}
	sortCandidates(cands)
	return cands, nil
}

// sortCandidates orders by ascending load, then ascending user id.
func sortCandidates(cands []candidate) {
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].Load != cands[j].Load {
			return cands[i].Load < cands[j].Load
		}
		return cands[i].UserID < cands[j].UserID
	})
}

// evaluateAgent validates that a specific agent is eligible for a sector,
// returning a precise error when not. allowOffline relaxes ONLY the availability
// checks (presence missing → "offline"; status != available): it is true for
// MANUAL routing (Assign/Transfer), where assigning to an offline agent is
// legitimate (advance distribution, supervisor assignment, agent who forgot to go
// online — they see the conversation when they return). It is false for automatic
// routing, which must never park a conversation on someone offline. Membership
// (sector), active status and capacity are ALWAYS enforced.
func (s *Service) evaluateAgent(ctx context.Context, agentID, sectorID string, allowOffline bool) error {
	u, err := s.users.FindByID(ctx, agentID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return apperror.Validation("agent not found").WithDetails(map[string]any{"agent_id": "not found"})
		}
		return err
	}
	if !u.IsActive() {
		return apperror.Validation("agent is disabled")
	}
	if !contains(u.SectorIDs, sectorID) {
		return apperror.Validation("agent does not belong to the sector")
	}
	if !allowOffline {
		p, err := s.presence.Get(ctx, agentID)
		if err != nil {
			return apperror.Conflict("agent is offline")
		}
		if p.Status != presenceentity.StatusAvailable {
			return apperror.Conflict("agent is not available")
		}
	}
	load, err := s.load.CountOpenAssigned(ctx, agentID)
	if err != nil {
		return err
	}
	if u.MaxConcurrentChats > 0 && load >= u.MaxConcurrentChats {
		return apperror.Conflict("agent is at maximum concurrent chats")
	}
	return nil
}

// ── internals ────────────────────────────────────────────────────────────────

// withLock loads the conversation, enforces actor access, takes the routing lock
// and runs fn against a freshly reloaded conversation. The lock serializes all
// routing operations on a conversation across nodes, preventing double
// assignment.
func (s *Service) withLock(ctx context.Context, conversationID string, fn func(*conventity.Conversation) (*conventity.Conversation, error)) (*conventity.Conversation, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	conv, err := s.conversations.FindByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if err := s.assertAccess(ctx, conv); err != nil {
		return nil, err
	}

	key := "routing:lock:" + tenantID + ":" + conversationID
	release, acquired, err := s.locker.Acquire(ctx, key, lockTTL)
	if err != nil {
		return nil, apperror.Internal("could not acquire routing lock").Wrap(err)
	}
	if !acquired {
		return nil, apperror.Conflict("conversation is being routed by another operation")
	}
	defer release()

	// Reload inside the lock so decisions use the latest state.
	conv, err = s.conversations.FindByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	return fn(conv)
}

// applyAssignment sets the conversation as assigned to agentID and emits the
// event + realtime.
func (s *Service) applyAssignment(ctx context.Context, conv *conventity.Conversation, agentID string) (*conventity.Conversation, error) {
	fromQueue := conv.QueueID
	conv.AssignedTo = agentID
	conv.Status = conventity.StatusAssigned
	conv.QueueID = ""
	conv.UpdatedAt = s.clock.Now()
	if err := s.conversations.Update(ctx, conv); err != nil {
		return nil, err
	}

	s.recordEvent(ctx, conv, conventity.EventConversationAssigned, map[string]any{"agent_id": agentID})
	s.publishAssigned(ctx, conv, agentID)
	// The conversation left the queue (waiting--) and is now assigned (assigned++).
	s.queueStats.QueueChanged(ctx, conv.SectorID, fromQueue)
	s.emitConversationWebhook(ctx, conv, conventity.EventConversationAssigned)
	s.notifier.Notify(ctx, shared.NotifyInput{
		TenantID: conv.TenantID, UserID: agentID,
		Type:  "conversation.assigned_to_you",
		Title: "A conversation was assigned to you",
		Link:  "/conversations/" + conv.ID,
	})
	return conv, nil
}

func (s *Service) recordEvent(ctx context.Context, conv *conventity.Conversation, eventType string, data map[string]any) {
	actorType := conventity.ActorSystem
	actorID := ""
	if ac, ok := authz.FromContext(ctx); ok && ac.UserID != "" {
		actorType = conventity.ActorAgent
		actorID = ac.UserID
	}
	_ = s.events.Create(ctx, &conventity.ConversationEvent{
		ID:             shared.NewID(),
		TenantID:       conv.TenantID,
		ConversationID: conv.ID,
		Type:           eventType,
		ActorType:      actorType,
		ActorID:        actorID,
		Data:           data,
		CreatedAt:      s.clock.Now(),
	})
}

func (s *Service) publishAssigned(ctx context.Context, conv *conventity.Conversation, agentID string) {
	payload := convcontracts.NewConversationPayload(conv)
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID), convcontracts.RealtimeConversationAssigned, payload)
	if conv.SectorID != "" {
		_ = s.publisher.Publish(ctx, shared.TopicInbox(conv.TenantID, conv.SectorID), convcontracts.RealtimeConversationAssigned, payload)
	}
	if agentID != "" {
		_ = s.publisher.Publish(ctx, shared.TopicUser(conv.TenantID, agentID), convcontracts.RealtimeConversationAssigned, payload)
	}
}

func (s *Service) publishTransferred(ctx context.Context, conv *conventity.Conversation, fromSector string) {
	payload := convcontracts.NewConversationPayload(conv)
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID), convcontracts.RealtimeConversationTransferred, payload)
	if fromSector != "" {
		_ = s.publisher.Publish(ctx, shared.TopicInbox(conv.TenantID, fromSector), convcontracts.RealtimeConversationTransferred, payload)
	}
	if conv.SectorID != "" && conv.SectorID != fromSector {
		_ = s.publisher.Publish(ctx, shared.TopicInbox(conv.TenantID, conv.SectorID), convcontracts.RealtimeConversationTransferred, payload)
	}
	if conv.AssignedTo != "" {
		_ = s.publisher.Publish(ctx, shared.TopicUser(conv.TenantID, conv.AssignedTo), convcontracts.RealtimeConversationTransferred, payload)
	}
}

func (s *Service) publishUpdated(ctx context.Context, conv *conventity.Conversation) {
	payload := convcontracts.NewConversationPayload(conv)
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID), convcontracts.RealtimeConversationUpdated, payload)
	if conv.SectorID != "" {
		_ = s.publisher.Publish(ctx, shared.TopicInbox(conv.TenantID, conv.SectorID), convcontracts.RealtimeConversationUpdated, payload)
	}
}

// assertAccess enforces the actor's visibility on the conversation (mirrors the
// conversations domain): all-scope sees everything; otherwise own sector or
// assigned. Hidden conversations return not_found.
func (s *Service) assertAccess(ctx context.Context, conv *conventity.Conversation) error {
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return apperror.Unauthorized("authentication required")
	}
	if ac.SectorScope == authz.ScopeAll {
		return nil
	}
	if conv.AssignedTo != "" && conv.AssignedTo == ac.UserID {
		return nil
	}
	if contains(ac.SectorIDs, conv.SectorID) && conv.SectorID != "" {
		return nil
	}
	return apperror.NotFound("conversation not found")
}

func (s *Service) visibility(ctx context.Context) (convcontracts.Visibility, error) {
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return convcontracts.Visibility{}, apperror.Unauthorized("authentication required")
	}
	return convcontracts.Visibility{
		All:       ac.SectorScope == authz.ScopeAll,
		SectorIDs: ac.SectorIDs,
		UserID:    ac.UserID,
	}, nil
}

// contains reports whether v is a non-empty member of ss. Empty entries are
// ignored defensively: a junk "" in a user's sector_ids must never be treated as
// belonging to a (real, non-empty) sector id.
func contains(ss []string, v string) bool {
	if v == "" {
		return false
	}
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
