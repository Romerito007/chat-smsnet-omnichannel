// Package service holds the conversations business logic: conversation
// lifecycle, messages, internal notes, timeline events, agent visibility and
// realtime fan-out.
package service

import (
	"context"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	sectorrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service implements the conversations use cases.
type Service struct {
	conversations repository.ConversationRepository
	messages      repository.MessageRepository
	events        repository.EventRepository
	sectors       sectorrepo.SectorRepository
	publisher     shared.EventPublisher
	clock         shared.Clock
	outbound      contracts.OutboundDispatcher
	webhooks      shared.WebhookEmitter
	ruleSink      shared.RuleEventSink
	tags          contracts.TagCatalog
	closeReasons  contracts.CloseReasonPolicy
	sla           contracts.SLAHook
	notifier      shared.Notifier
	csat          contracts.CSATTrigger
	auditor       shared.Auditor
	queueStats    shared.QueueStatsNotifier
	attachments   contracts.AttachmentResolver
	contacts      contracts.ContactDirectory
	agents        contracts.AgentDirectory
}

// SetAuditor wires the audit trail. Optional: when unset, conversation closes are
// not audited.
func (s *Service) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// SetAttachmentResolver wires the attachments hydrator/validator so message
// attachments are returned with full metadata (url/content_type/filename/size)
// and validated on send. Optional: when unset, attachments pass through id-only.
func (s *Service) SetAttachmentResolver(a contracts.AttachmentResolver) {
	if a != nil {
		s.attachments = a
	}
}

// SetContactDirectory wires the resolver of contact display cards (name + signed
// avatar URL) for the inbox rows. Optional.
func (s *Service) SetContactDirectory(d contracts.ContactDirectory) {
	if d != nil {
		s.contacts = d
	}
}

// SetAgentDirectory wires the resolver of agent (assignee) display cards. Optional.
func (s *Service) SetAgentDirectory(d contracts.AgentDirectory) {
	if d != nil {
		s.agents = d
	}
}

// ContactCards batch-resolves the contact display cards for a page of
// conversations, keyed by contact id. Best-effort and nil-safe.
func (s *Service) ContactCards(ctx context.Context, conversations []*entity.Conversation) (map[string]shared.DisplayCard, error) {
	if s.contacts == nil {
		return nil, nil
	}
	ids := dedupeField(conversations, func(c *entity.Conversation) string { return c.ContactID })
	if len(ids) == 0 {
		return nil, nil
	}
	return s.contacts.ContactCards(ctx, ids)
}

// AgentCards batch-resolves the assignee display cards for a page of
// conversations, keyed by user id. Best-effort and nil-safe.
func (s *Service) AgentCards(ctx context.Context, conversations []*entity.Conversation) (map[string]shared.DisplayCard, error) {
	if s.agents == nil {
		return nil, nil
	}
	ids := dedupeField(conversations, func(c *entity.Conversation) string { return c.AssignedTo })
	if len(ids) == 0 {
		return nil, nil
	}
	return s.agents.AgentCards(ctx, ids)
}

// dedupeField collects the non-empty, de-duplicated values of a field across the
// page, preserving order.
func dedupeField(conversations []*entity.Conversation, field func(*entity.Conversation) string) []string {
	ids := make([]string, 0, len(conversations))
	seen := make(map[string]struct{}, len(conversations))
	for _, c := range conversations {
		v := field(c)
		if v == "" {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		ids = append(ids, v)
	}
	return ids
}

// SetQueueStatsNotifier wires the queue.stats notifier. Optional: when unset,
// queue-composition changes are not broadcast.
func (s *Service) SetQueueStatsNotifier(n shared.QueueStatsNotifier) {
	if n != nil {
		s.queueStats = n
	}
}

// SetNotifier wires the user notifier. Optional: when unset, mentions do not
// notify.
func (s *Service) SetNotifier(n shared.Notifier) {
	if n != nil {
		s.notifier = n
	}
}

// SetCSATTrigger wires the CSAT close trigger. Optional: when unset, closing a
// conversation does not start a survey.
func (s *Service) SetCSATTrigger(t contracts.CSATTrigger) {
	if t != nil {
		s.csat = t
	}
}

// SetSLAHook wires the SLA lifecycle hook. Optional: when unset, no SLA tracking
// is created or advanced.
func (s *Service) SetSLAHook(h contracts.SLAHook) {
	if h != nil {
		s.sla = h
	}
}

// SetOutboundDispatcher wires the channels delivery dispatcher. Optional: when
// unset, outbound messages are persisted (pending) but not delivered.
func (s *Service) SetOutboundDispatcher(d contracts.OutboundDispatcher) {
	s.outbound = d
}

// SetTagCatalog wires the conversationtools tag catalog used to validate tag ids
// on apply. Optional: when unset, tag ids are accepted as-is.
func (s *Service) SetTagCatalog(t contracts.TagCatalog) {
	s.tags = t
}

// SetCloseReasonPolicy wires the conversationtools close-reason policy used to
// enforce requires_note on Close. Optional: when unset, no note is required.
func (s *Service) SetCloseReasonPolicy(p contracts.CloseReasonPolicy) {
	s.closeReasons = p
}

// SetWebhookEmitter wires the outbound webhook emitter. Optional: when unset,
// business events are not forwarded to webhook subscriptions.
func (s *Service) SetWebhookEmitter(e shared.WebhookEmitter) {
	if e != nil {
		s.webhooks = e
	}
}

// SetRuleEventSink wires the automation-rules sink. Optional: when unset, lifecycle
// events are not evaluated by automation rules. Emission is async (enqueue only).
func (s *Service) SetRuleEventSink(sink shared.RuleEventSink) {
	if sink != nil {
		s.ruleSink = sink
	}
}

// emitRuleEvent forwards a lifecycle event to the automation-rules sink (best
// effort, async). event is the internal dot-notation name; payload is the event's
// conversation/message payload (used as the webhook data).
func (s *Service) emitRuleEvent(ctx context.Context, conv *entity.Conversation, event string, payload any) {
	if s.ruleSink != nil {
		s.ruleSink.EmitRuleEvent(ctx, conv.TenantID, event, conv.ID, payload)
	}
}

// New builds the service.
func New(
	conversations repository.ConversationRepository,
	messages repository.MessageRepository,
	events repository.EventRepository,
	sectors sectorrepo.SectorRepository,
	publisher shared.EventPublisher,
	clock shared.Clock,
) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if publisher == nil {
		publisher = shared.NoopPublisher{}
	}
	return &Service{
		conversations: conversations,
		messages:      messages,
		events:        events,
		sectors:       sectors,
		publisher:     publisher,
		clock:         clock,
		webhooks:      shared.NoopWebhookEmitter{},
		sla:           contracts.NoopSLAHook{},
		notifier:      shared.NoopNotifier{},
		csat:          contracts.NoopCSATTrigger{},
		auditor:       shared.NoopAuditor{},
		queueStats:    shared.NoopQueueStatsNotifier{},
	}
}

// Create opens a conversation.
func (s *Service) Create(ctx context.Context, cmd contracts.CreateConversation) (*entity.Conversation, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}

	v := map[string]any{}
	contactID := strings.TrimSpace(cmd.ContactID)
	if contactID == "" {
		v["contact_id"] = "is required"
	}
	channel := strings.TrimSpace(cmd.Channel)
	if channel == "" {
		v["channel"] = "is required"
	}
	priority := cmd.Priority
	if priority == "" {
		priority = entity.PriorityNormal
	}
	if !priority.Valid() {
		v["priority"] = "must be one of low|normal|high|urgent"
	}
	if len(v) > 0 {
		return nil, apperror.Validation("validation failed").WithDetails(v)
	}

	// Validate the sector exists within the tenant when provided.
	if cmd.SectorID != "" {
		if _, err := s.sectors.FindByID(ctx, cmd.SectorID); err != nil {
			if apperror.From(err).Code == apperror.CodeNotFound {
				return nil, apperror.Validation("sector does not exist").
					WithDetails(map[string]any{"sector_id": "not found"})
			}
			return nil, err
		}
	}

	now := s.clock.Now()
	status := entity.StatusNew
	if cmd.AssignedTo != "" {
		status = entity.StatusAssigned
	} else if cmd.QueueID != "" {
		status = entity.StatusQueued
	}

	conv := &entity.Conversation{
		ID:            shared.NewID(),
		TenantID:      tenantID,
		ContactID:     contactID,
		Channel:       channel,
		ChannelID:     strings.TrimSpace(cmd.ChannelID),
		SectorID:      cmd.SectorID,
		QueueID:       cmd.QueueID,
		Status:        status,
		AssignedTo:    cmd.AssignedTo,
		Priority:      priority,
		Tags:          cmd.Tags,
		LastMessageAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.conversations.Create(ctx, conv); err != nil {
		return nil, err
	}

	s.recordEvent(ctx, conv, entity.EventConversationCreated, nil)
	s.publishConversation(ctx, conv)
	s.publishLifecycle(ctx, conv, contracts.RealtimeConversationCreated)
	s.webhooks.Emit(ctx, conv.TenantID, entity.EventConversationCreated, conv.SectorID, contracts.NewConversationPayload(conv))
	s.sla.OnConversationCreated(ctx, conv)
	if conv.Status == entity.StatusQueued && conv.QueueID != "" {
		s.queueStats.QueueChanged(ctx, conv.SectorID, conv.QueueID) // entered the queue
	}
	return conv, nil
}

// Get returns a conversation the actor is allowed to see.
func (s *Service) Get(ctx context.Context, id string) (*entity.Conversation, error) {
	conv, _, err := s.loadVisible(ctx, id)
	return conv, err
}

// List returns conversations matching the filter, scoped to the actor's
// visibility (own sectors / assigned, unless the role has all-sector scope).
func (s *Service) List(ctx context.Context, filter contracts.ListFilter, page shared.PageRequest) ([]*entity.Conversation, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	vis, err := s.visibility(ctx)
	if err != nil {
		return nil, err
	}
	return s.conversations.List(ctx, filter, vis, page.Normalize())
}

// LastMessages returns the latest non-deleted message per conversation id (for
// list-item previews). Conversations with an empty thread are absent from the
// map. Tenant-scoped; the caller already authorized the listing.
func (s *Service) LastMessages(ctx context.Context, conversationIDs []string) (map[string]*entity.Message, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	// Single aggregation across the page (no per-conversation query).
	return s.messages.LatestByConversations(ctx, conversationIDs)
}

// Update applies the non-nil fields of cmd.
func (s *Service) Update(ctx context.Context, id string, cmd contracts.UpdateConversation) (*entity.Conversation, error) {
	conv, _, err := s.loadVisible(ctx, id)
	if err != nil {
		return nil, err
	}

	if cmd.SectorID != nil {
		conv.SectorID = *cmd.SectorID
	}
	if cmd.QueueID != nil {
		conv.QueueID = *cmd.QueueID
	}
	if cmd.AssignedTo != nil {
		conv.AssignedTo = *cmd.AssignedTo
	}
	if cmd.Priority != nil {
		if !cmd.Priority.Valid() {
			return nil, apperror.Validation("invalid priority")
		}
		conv.Priority = *cmd.Priority
	}
	statusChanged := false
	if cmd.Status != nil {
		if !cmd.Status.Valid() {
			return nil, apperror.Validation("invalid status")
		}
		statusChanged = conv.Status != *cmd.Status
		conv.Status = *cmd.Status
	}
	if cmd.Tags != nil {
		conv.Tags = *cmd.Tags
	}
	conv.UpdatedAt = s.clock.Now()
	if err := s.conversations.Update(ctx, conv); err != nil {
		return nil, err
	}

	s.recordEvent(ctx, conv, entity.EventConversationUpdated, nil)
	s.publishConversation(ctx, conv)
	// conversation.updated drives automation rules only from an explicit Update
	// (PATCH), not from every internal publishConversation caller.
	s.emitRuleEvent(ctx, conv, contracts.RealtimeConversationUpdated, contracts.NewConversationPayload(conv))
	// A status change that lands on a terminal state also emits the named
	// lifecycle event (e.g. resolving a conversation via PATCH status=resolved).
	if statusChanged {
		s.publishLifecycle(ctx, conv, lifecycleEventFor(conv.Status))
	}
	return conv, nil
}

// SendMessage posts an outbound message from the acting agent. Outbound messages
// are born delivery_status=pending; the channels domain performs delivery.
func (s *Service) SendMessage(ctx context.Context, conversationID string, cmd contracts.SendMessage) (*entity.Message, error) {
	conv, ac, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if conv.Status.IsClosed() {
		return nil, apperror.Conflict("conversation is closed")
	}

	mtype := cmd.MessageType
	if mtype == "" {
		mtype = entity.MessageText
	}
	if !mtype.Valid() {
		return nil, apperror.Validation("invalid message_type")
	}
	if strings.TrimSpace(cmd.Text) == "" && len(cmd.Attachments) == 0 {
		return nil, apperror.Validation("message text or attachments required")
	}
	// Reject a send that references a non-existent / cross-tenant / unconfirmed
	// attachment, so we never store an orphan reference.
	if s.attachments != nil && len(cmd.Attachments) > 0 {
		if err := s.attachments.ValidateMessageAttachments(ctx, attachmentIDs(cmd.Attachments)); err != nil {
			return nil, err
		}
	}

	now := s.clock.Now()
	msg := &entity.Message{
		ID:             shared.NewID(),
		TenantID:       conv.TenantID,
		ConversationID: conv.ID,
		SenderType:     entity.SenderAgent,
		SenderID:       ac.UserID,
		Direction:      entity.DirectionOutbound,
		MessageType:    mtype,
		Text:           cmd.Text,
		Attachments:    cmd.Attachments,
		Metadata:       cmd.Metadata,
		CreatedAt:      now,
		DeliveryStatus: entity.DeliveryPending,
	}
	saved, err := s.persistMessage(ctx, conv, msg, entity.EventMessageCreated)
	if err != nil {
		return nil, err
	}
	// Hand off to the channels domain for delivery (best-effort: a channel
	// failure must not fail the agent's send).
	if s.outbound != nil {
		s.outbound.Dispatch(ctx, conv, saved)
	}
	// SLA: an agent's outbound message is the first response (idempotent).
	s.sla.OnFirstResponse(ctx, conv, saved.CreatedAt)
	return saved, nil
}

// AddInternalNote adds an internal note that is never delivered to the customer.
func (s *Service) AddInternalNote(ctx context.Context, conversationID string, cmd contracts.AddInternalNote) (*entity.Message, error) {
	conv, ac, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.Text) == "" {
		return nil, apperror.Validation("note text is required")
	}

	now := s.clock.Now()
	msg := &entity.Message{
		ID:             shared.NewID(),
		TenantID:       conv.TenantID,
		ConversationID: conv.ID,
		SenderType:     entity.SenderAgent,
		SenderID:       ac.UserID,
		Direction:      entity.DirectionInternal,
		MessageType:    entity.MessageText,
		Text:           cmd.Text,
		CreatedAt:      now,
		DeliveryStatus: entity.DeliveryNone,
	}
	saved, err := s.persistMessage(ctx, conv, msg, entity.EventInternalNoteAdded)
	if err != nil {
		return nil, err
	}
	// Notify @-mentioned users (never the author).
	for _, uid := range cmd.MentionUserIDs {
		if uid == "" || uid == ac.UserID {
			continue
		}
		s.notifier.Notify(ctx, shared.NotifyInput{
			TenantID: conv.TenantID, UserID: uid,
			Type:  "mention.internal_note",
			Title: "You were mentioned in an internal note",
			Link:  "/conversations/" + conv.ID,
		})
	}
	return saved, nil
}

// Close closes a conversation, optionally recording a close reason and note.
func (s *Service) Close(ctx context.Context, conversationID string, cmd contracts.CloseConversation) (*entity.Conversation, error) {
	conv, _, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if conv.Status == entity.StatusClosed || conv.Status == entity.StatusArchived {
		return nil, apperror.Conflict("conversation is already closed")
	}

	// Enforce the close-reason policy: a reason that requires a note cannot be
	// used to close without one.
	if cmd.CloseReasonID != "" && s.closeReasons != nil {
		requiresNote, rerr := s.closeReasons.RequiresNote(ctx, cmd.CloseReasonID)
		if rerr != nil {
			return nil, rerr
		}
		if requiresNote && strings.TrimSpace(cmd.Note) == "" {
			return nil, apperror.Validation("this close reason requires a note").
				WithDetails(map[string]any{"note": "is required for this close reason"})
		}
	}

	// Capture queue membership before closing so we can refresh queue.stats.
	wasQueued := conv.Status == entity.StatusQueued && conv.QueueID != ""

	now := s.clock.Now()
	conv.Status = entity.StatusClosed
	conv.ClosedAt = &now
	conv.UpdatedAt = now
	if err := s.conversations.Update(ctx, conv); err != nil {
		return nil, err
	}

	if strings.TrimSpace(cmd.Note) != "" {
		if _, err := s.AddInternalNote(ctx, conv.ID, contracts.AddInternalNote{Text: cmd.Note}); err != nil {
			return nil, err
		}
	}

	s.recordEvent(ctx, conv, entity.EventConversationClosed, map[string]any{
		"close_reason_id": cmd.CloseReasonID,
		"note":            cmd.Note,
	})
	s.publishConversation(ctx, conv)
	s.publishLifecycle(ctx, conv, contracts.RealtimeConversationClosed)
	s.webhooks.Emit(ctx, conv.TenantID, entity.EventConversationClosed, conv.SectorID, contracts.NewConversationPayload(conv))
	s.sla.OnResolved(ctx, conv, now)
	s.csat.OnConversationClosed(ctx, conv)
	if wasQueued {
		s.queueStats.QueueChanged(ctx, conv.SectorID, conv.QueueID) // left the queue
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "conversation.closed", ResourceType: "conversation", ResourceID: conv.ID,
		Data: map[string]any{"close_reason_id": cmd.CloseReasonID},
	})
	return conv, nil
}

// CloseInactive closes every non-closed conversation in the current tenant whose
// last activity is older than idleFor, recording the close event + realtime and
// running the close side-effects (webhook, SLA resolve, CSAT). It is idempotent:
// an already-closed conversation is not selected again. Returns the count closed.
// Run by the chat.close_inactive_conversations job as a system actor.
func (s *Service) CloseInactive(ctx context.Context, idleFor time.Duration) (int, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return 0, err
	}
	now := s.clock.Now()
	convs, err := s.conversations.ListInactiveOpen(ctx, now.Add(-idleFor), 500)
	if err != nil {
		return 0, err
	}
	closed := 0
	for _, conv := range convs {
		wasQueued := conv.Status == entity.StatusQueued && conv.QueueID != ""
		conv.Status = entity.StatusClosed
		conv.ClosedAt = &now
		conv.UpdatedAt = now
		if err := s.conversations.Update(ctx, conv); err != nil {
			continue // best-effort; next run retries
		}
		s.recordEvent(ctx, conv, entity.EventConversationClosed, map[string]any{"reason": "inactivity"})
		s.publishConversation(ctx, conv)
		s.publishLifecycle(ctx, conv, contracts.RealtimeConversationClosed)
		s.webhooks.Emit(ctx, conv.TenantID, entity.EventConversationClosed, conv.SectorID, contracts.NewConversationPayload(conv))
		s.sla.OnResolved(ctx, conv, now)
		s.csat.OnConversationClosed(ctx, conv)
		if wasQueued {
			s.queueStats.QueueChanged(ctx, conv.SectorID, conv.QueueID)
		}
		closed++
	}
	return closed, nil
}

// SendSystemMessage sends a system-authored outbound message to a conversation's
// channel, bypassing the closed-conversation guard (used e.g. to deliver a CSAT
// survey after the conversation is closed). It is tenant-scoped from ctx and does
// not enforce agent visibility — callers are trusted system actors.
func (s *Service) SendSystemMessage(ctx context.Context, conversationID, text string) (*entity.Message, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	conv, err := s.conversations.FindByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(text) == "" {
		return nil, apperror.Validation("message text is required")
	}
	now := s.clock.Now()
	msg := &entity.Message{
		ID:             shared.NewID(),
		TenantID:       conv.TenantID,
		ConversationID: conv.ID,
		SenderType:     entity.SenderSystem,
		Direction:      entity.DirectionOutbound,
		MessageType:    entity.MessageText,
		Text:           text,
		CreatedAt:      now,
		DeliveryStatus: entity.DeliveryPending,
	}
	saved, err := s.persistMessage(ctx, conv, msg, entity.EventMessageCreated)
	if err != nil {
		return nil, err
	}
	if s.outbound != nil {
		s.outbound.Dispatch(ctx, conv, saved)
	}
	return saved, nil
}

// ApplyTags adds and/or removes tags on a conversation. Added tags are validated
// against the tenant's tag catalog (when wired). It records a conversation.tagged
// timeline event and publishes realtime conversation.tagged + conversation.updated.
func (s *Service) ApplyTags(ctx context.Context, conversationID string, add, remove []string) (*entity.Conversation, error) {
	conv, _, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	add = dedupe(add)
	remove = dedupe(remove)
	if len(add) == 0 && len(remove) == 0 {
		return nil, apperror.Validation("provide at least one tag to add or remove")
	}
	// Canonicalize to ids so conv.Tags is ALWAYS ids (never a name): add is strict
	// (unknown tag -> 400), remove is lenient (a stale value still gets stripped).
	if s.tags != nil {
		if len(add) > 0 {
			resolved, err := s.tags.ResolveTags(ctx, add, true)
			if err != nil {
				return nil, err
			}
			add = resolved
		}
		if len(remove) > 0 {
			resolved, err := s.tags.ResolveTags(ctx, remove, false)
			if err != nil {
				return nil, err
			}
			remove = resolved
		}
	}

	conv.Tags = applyTagChanges(conv.Tags, add, remove)
	conv.UpdatedAt = s.clock.Now()
	if err := s.conversations.Update(ctx, conv); err != nil {
		return nil, err
	}

	s.recordEvent(ctx, conv, entity.EventConversationTagged, map[string]any{"added": add, "removed": remove})
	payload := contracts.NewConversationPayload(conv)
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID), contracts.RealtimeConversationTagged, payload)
	s.publishConversation(ctx, conv)
	return conv, nil
}

// applyTagChanges returns current with add merged in and remove taken out,
// preserving order and de-duplicating.
func applyTagChanges(current, add, remove []string) []string {
	removeSet := make(map[string]struct{}, len(remove))
	for _, r := range remove {
		removeSet[r] = struct{}{}
	}
	seen := make(map[string]struct{}, len(current)+len(add))
	out := make([]string, 0, len(current)+len(add))
	keep := func(t string) {
		if t == "" {
			return
		}
		if _, drop := removeSet[t]; drop {
			return
		}
		if _, dup := seen[t]; dup {
			return
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	for _, t := range current {
		keep(t)
	}
	for _, t := range add {
		keep(t)
	}
	return out
}

func dedupe(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// Reopen reopens a closed/resolved/archived conversation.
func (s *Service) Reopen(ctx context.Context, conversationID string) (*entity.Conversation, error) {
	conv, _, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if !conv.Status.IsClosed() {
		return nil, apperror.Conflict("conversation is not closed")
	}

	conv.Status = entity.StatusNew
	if conv.AssignedTo != "" {
		conv.Status = entity.StatusAssigned
	} else if conv.QueueID != "" {
		conv.Status = entity.StatusQueued
	}
	conv.ClosedAt = nil
	conv.UpdatedAt = s.clock.Now()
	if err := s.conversations.Update(ctx, conv); err != nil {
		return nil, err
	}

	s.recordEvent(ctx, conv, entity.EventConversationReopened, nil)
	s.publishConversation(ctx, conv)
	s.publishLifecycle(ctx, conv, contracts.RealtimeConversationReopened)
	return conv, nil
}

// ListMessages returns the message timeline of a visible conversation.
func (s *Service) ListMessages(ctx context.Context, conversationID string, page shared.PageRequest) ([]*entity.Message, error) {
	if _, _, err := s.loadVisible(ctx, conversationID); err != nil {
		return nil, err
	}
	msgs, err := s.messages.ListByConversation(ctx, conversationID, page.Normalize())
	if err != nil {
		return nil, err
	}
	s.hydrateMessages(ctx, msgs...)
	return msgs, nil
}

// ListEvents returns the conversation timeline (lifecycle/automation events),
// which are persisted separately from chat messages. Cursor-paginated.
func (s *Service) ListEvents(ctx context.Context, conversationID string, page shared.PageRequest) ([]*entity.ConversationEvent, error) {
	if _, _, err := s.loadVisible(ctx, conversationID); err != nil {
		return nil, err
	}
	return s.events.ListByConversation(ctx, conversationID, page.Normalize())
}

// SetTyping publishes a typing.started/stopped event to the conversation room.
// Typing is ephemeral (not persisted); it only requires the actor to see the
// conversation.
func (s *Service) SetTyping(ctx context.Context, conversationID string, on bool) error {
	conv, ac, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return err
	}
	event := contracts.RealtimeTypingStopped
	if on {
		event = contracts.RealtimeTypingStarted
	}
	return s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID), event,
		contracts.TypingPayload{ConversationID: conv.ID, UserID: ac.UserID})
}

// MarkRead records that the actor read the conversation: it zeroes the unread
// counter, stamps last_read_at, and publishes both a message.read receipt and a
// conversation.updated (so the inbox reflects the cleared badge).
func (s *Service) MarkRead(ctx context.Context, conversationID string) error {
	conv, ac, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	if conv.UnreadCount != 0 || conv.LastReadAt == nil {
		conv.UnreadCount = 0
		conv.LastReadAt = &now
		conv.UpdatedAt = now
		if err := s.conversations.Update(ctx, conv); err != nil {
			return err
		}
		s.publishConversation(ctx, conv)
	}
	return s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID), contracts.RealtimeMessageRead,
		contracts.ReadPayload{ConversationID: conv.ID, UserID: ac.UserID, ReadAt: now})
}

// EditMessage edits a message's text (soft edit). It sets edited_at and keeps the
// message in place — only agent-authored messages can be edited, and only by the
// author or someone holding message.delete (the elevated "manage messages"
// capability). Editing never touches the external channel: a message already
// delivered to the customer is only updated in the chat. Publishes message.updated.
func (s *Service) EditMessage(ctx context.Context, conversationID, messageID string, cmd contracts.EditMessage) (*entity.Message, error) {
	conv, ac, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(cmd.Text)
	if text == "" {
		return nil, apperror.Validation("message text is required")
	}
	msg, err := s.loadMessage(ctx, conv, messageID)
	if err != nil {
		return nil, err
	}
	// Only agent-authored messages (outbound replies or internal notes) are
	// editable; a customer's words are never rewritten.
	if msg.SenderType != entity.SenderAgent {
		return nil, apperror.Validation("only agent messages can be edited")
	}
	if !s.canManageMessage(ac, msg) {
		return nil, apperror.Forbidden("you can only edit your own messages")
	}

	now := s.clock.Now()
	msg.Text = text
	msg.EditedAt = &now
	if err := s.messages.Update(ctx, msg); err != nil {
		return nil, err
	}
	// Hydrate attachments for the realtime payload and the response.
	s.hydrateMessages(ctx, msg)

	s.recordEvent(ctx, conv, entity.EventMessageEdited, map[string]any{"message_id": msg.ID})
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID),
		contracts.RealtimeMessageUpdated, contracts.NewMessagePayload(msg))
	return msg, nil
}

// DeleteMessage soft-deletes a message: it sets deleted_at so the message
// disappears from listings while staying in the database (history preserved).
// The route requires message.delete; the elevated holder may delete any message,
// including another user's internal note. A message already delivered to the
// customer is only soft-marked in the chat — the external channel is never
// touched. Idempotent. Publishes message.deleted.
func (s *Service) DeleteMessage(ctx context.Context, conversationID, messageID string) error {
	conv, ac, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return err
	}
	msg, err := s.loadMessage(ctx, conv, messageID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeConflict {
			return nil // already deleted → idempotent
		}
		return err
	}
	if !s.canManageMessage(ac, msg) {
		return apperror.Forbidden("not allowed to delete this message")
	}

	now := s.clock.Now()
	msg.DeletedAt = &now
	if err := s.messages.Update(ctx, msg); err != nil {
		return err
	}

	s.recordEvent(ctx, conv, entity.EventMessageDeleted, map[string]any{"message_id": msg.ID})
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID),
		contracts.RealtimeMessageDeleted, contracts.MessageRefPayload{MessageID: msg.ID, ConversationID: conv.ID})
	// Audit the deletion: the sender_type lets reviewers tell content moderation of
	// a customer message apart from an agent retracting their own. actor_id/type/ip/
	// user_agent are filled from the request context by the recorder.
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "message.deleted", ResourceType: "message", ResourceID: msg.ID,
		Data: map[string]any{"sender_type": string(msg.SenderType), "conversation_id": conv.ID},
	})
	return nil
}

// loadMessage fetches a message and verifies it belongs to the conversation and
// is not already soft-deleted (conflict, so callers can treat delete idempotently).
func (s *Service) loadMessage(ctx context.Context, conv *entity.Conversation, messageID string) (*entity.Message, error) {
	msg, err := s.messages.FindByID(ctx, messageID)
	if err != nil {
		return nil, err
	}
	if msg.ConversationID != conv.ID {
		return nil, apperror.NotFound("message not found")
	}
	if msg.IsDeleted() {
		return nil, apperror.Conflict("message is already deleted")
	}
	return msg, nil
}

// canManageMessage reports whether the actor may edit/delete the message: the
// author always may; otherwise the elevated message.delete permission is required
// (this also guards "do not edit/delete another user's internal note").
func (s *Service) canManageMessage(ac authz.AuthContext, msg *entity.Message) bool {
	if msg.SenderID != "" && msg.SenderID == ac.UserID {
		return true
	}
	return ac.Has(authz.MessageDelete)
}

// ── internals ────────────────────────────────────────────────────────────────

// persistMessage stores a message, bumps conversation activity, records the
// timeline event, and publishes realtime message.created + conversation.updated.
func (s *Service) persistMessage(ctx context.Context, conv *entity.Conversation, msg *entity.Message, eventType string) (*entity.Message, error) {
	// Resolve attachment metadata in-memory and derive message_type, then store
	// attachment IDS ONLY (persistence unchanged); the realtime payload and the
	// returned message carry the full, hydrated attachments.
	s.hydrateMessages(ctx, msg)
	hydrated := msg.Attachments
	msg.Attachments = attachmentIDsOnly(hydrated)
	if err := s.messages.Create(ctx, msg); err != nil {
		return nil, err
	}
	msg.Attachments = hydrated

	// Every message updates last activity on the conversation.
	conv.LastMessageAt = msg.CreatedAt
	conv.UpdatedAt = msg.CreatedAt
	if err := s.conversations.Update(ctx, conv); err != nil {
		return nil, err
	}

	s.recordEvent(ctx, conv, eventType, map[string]any{"message_id": msg.ID})

	// Realtime: message to the conversation topic; conversation update to inbox.
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID),
		contracts.RealtimeMessageCreated, contracts.NewMessagePayload(msg))
	s.publishConversation(ctx, conv)

	// Outbound webhook + automation rules: only real messages, not internal notes.
	if eventType == entity.EventMessageCreated {
		s.webhooks.Emit(ctx, conv.TenantID, entity.EventMessageCreated, conv.SectorID, contracts.NewMessagePayload(msg))
		s.emitRuleEvent(ctx, conv, entity.EventMessageCreated, contracts.NewMessagePayload(msg))
	}
	return msg, nil
}

// hydrateMessages fills each message's attachments with their full media metadata
// (url/content_type/filename/size) from the attachments domain — a single batch
// lookup across all messages (no N+1) — and derives message_type from the
// attachment when the message has an attachment and no text. Persistence is
// unchanged (the stored message keeps attachment ids); this is the read boundary.
func (s *Service) hydrateMessages(ctx context.Context, msgs ...*entity.Message) {
	if s.attachments == nil || len(msgs) == 0 {
		return
	}
	var ids []string
	for _, m := range msgs {
		for _, a := range m.Attachments {
			if a.ID != "" {
				ids = append(ids, a.ID)
			}
		}
	}
	if len(ids) == 0 {
		return
	}
	resolved, err := s.attachments.HydrateAttachments(ctx, ids)
	if err != nil || len(resolved) == 0 {
		return // best-effort: never fail a read because hydration hiccuped
	}
	for _, m := range msgs {
		for i, a := range m.Attachments {
			if full, ok := resolved[a.ID]; ok {
				m.Attachments[i] = full
			}
		}
		m.MessageType = deriveMessageType(m)
	}
}

// attachmentIDs returns the non-empty attachment ids of a message's attachments.
func attachmentIDs(atts []entity.Attachment) []string {
	ids := make([]string, 0, len(atts))
	for _, a := range atts {
		if a.ID != "" {
			ids = append(ids, a.ID)
		}
	}
	return ids
}

// attachmentIDsOnly reduces attachments to id-only references for persistence
// (metadata is rehydrated at the read boundary, never stored on the message).
func attachmentIDsOnly(atts []entity.Attachment) []entity.Attachment {
	if len(atts) == 0 {
		return atts
	}
	out := make([]entity.Attachment, len(atts))
	for i, a := range atts {
		out[i] = entity.Attachment{ID: a.ID}
	}
	return out
}

// deriveMessageType keeps an explicit text/template/system type, but for a media
// message with no text it derives the type from the (hydrated) first attachment's
// content type, so an attachment-only message is never reported as "text".
func deriveMessageType(m *entity.Message) entity.MessageType {
	if strings.TrimSpace(m.Text) != "" || len(m.Attachments) == 0 {
		return m.MessageType
	}
	switch m.MessageType {
	case entity.MessageTemplate, entity.MessageSystem:
		return m.MessageType // honor an explicit non-media type
	}
	return entity.MessageTypeForContentType(m.Attachments[0].ContentType)
}

// recordEvent appends a timeline event. Failures are swallowed (best-effort
// audit) so they never fail the primary operation.
func (s *Service) recordEvent(ctx context.Context, conv *entity.Conversation, eventType string, data map[string]any) {
	actorType := entity.ActorSystem
	actorID := ""
	if ac, ok := authz.FromContext(ctx); ok && ac.UserID != "" {
		actorType = entity.ActorAgent
		actorID = ac.UserID
	}
	_ = s.events.Create(ctx, &entity.ConversationEvent{
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

// publishConversation emits conversation.updated to the conversation topic and,
// when sectored, the sector inbox topic.
func (s *Service) publishConversation(ctx context.Context, conv *entity.Conversation) {
	payload := contracts.NewConversationPayload(conv)
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID),
		contracts.RealtimeConversationUpdated, payload)
	if conv.SectorID != "" {
		_ = s.publisher.Publish(ctx, shared.TopicInbox(conv.TenantID, conv.SectorID),
			contracts.RealtimeConversationUpdated, payload)
	}
}

// publishLifecycle publishes a named lifecycle event (conversation.created /
// .closed / .resolved / .reopened) alongside the generic conversation.updated, to
// the same conversation + inbox topics. A blank event is a no-op so callers can
// pass lifecycleEventFor's result directly.
func (s *Service) publishLifecycle(ctx context.Context, conv *entity.Conversation, event string) {
	if event == "" {
		return
	}
	payload := contracts.NewConversationPayload(conv)
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID), event, payload)
	if conv.SectorID != "" {
		_ = s.publisher.Publish(ctx, shared.TopicInbox(conv.TenantID, conv.SectorID), event, payload)
	}
	// Named lifecycle events (created/resolved/closed/reopened) drive automation rules.
	s.emitRuleEvent(ctx, conv, event, payload)
}

// lifecycleEventFor maps a terminal status to its named realtime lifecycle event.
// Returns "" for non-terminal statuses (handled by Create/Reopen explicitly).
func lifecycleEventFor(status entity.Status) string {
	switch status {
	case entity.StatusResolved:
		return contracts.RealtimeConversationResolved
	case entity.StatusClosed, entity.StatusArchived:
		return contracts.RealtimeConversationClosed
	default:
		return ""
	}
}

// visibility builds the actor's visibility from the AuthContext.
func (s *Service) visibility(ctx context.Context) (contracts.Visibility, error) {
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return contracts.Visibility{}, apperror.Unauthorized("authentication required")
	}
	return contracts.Visibility{
		All:       ac.SectorScope == authz.ScopeAll,
		SectorIDs: ac.SectorIDs,
		UserID:    ac.UserID,
	}, nil
}

// loadVisible loads a conversation and enforces the actor's visibility, returning
// not_found when the actor may not see it (avoiding existence leaks).
func (s *Service) loadVisible(ctx context.Context, id string) (*entity.Conversation, authz.AuthContext, error) {
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
	if !visibleTo(conv, ac) {
		return nil, ac, apperror.NotFound("conversation not found")
	}
	return conv, ac, nil
}

// visibleTo reports whether the actor may see the conversation.
func visibleTo(conv *entity.Conversation, ac authz.AuthContext) bool {
	if ac.SectorScope == authz.ScopeAll {
		return true
	}
	if conv.AssignedTo != "" && conv.AssignedTo == ac.UserID {
		return true
	}
	for _, sid := range ac.SectorIDs {
		if sid == conv.SectorID && sid != "" {
			return true
		}
	}
	return false
}
