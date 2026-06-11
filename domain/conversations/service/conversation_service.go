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
	tags          contracts.TagCatalog
	closeReasons  contracts.CloseReasonPolicy
	sla           contracts.SLAHook
	notifier      shared.Notifier
	csat          contracts.CSATTrigger
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
	s.webhooks.Emit(ctx, conv.TenantID, entity.EventConversationCreated, contracts.NewConversationPayload(conv))
	s.sla.OnConversationCreated(ctx, conv)
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
	if cmd.Status != nil {
		if !cmd.Status.Valid() {
			return nil, apperror.Validation("invalid status")
		}
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
	s.webhooks.Emit(ctx, conv.TenantID, entity.EventConversationClosed, contracts.NewConversationPayload(conv))
	s.sla.OnResolved(ctx, conv, now)
	s.csat.OnConversationClosed(ctx, conv)
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
		conv.Status = entity.StatusClosed
		conv.ClosedAt = &now
		conv.UpdatedAt = now
		if err := s.conversations.Update(ctx, conv); err != nil {
			continue // best-effort; next run retries
		}
		s.recordEvent(ctx, conv, entity.EventConversationClosed, map[string]any{"reason": "inactivity"})
		s.publishConversation(ctx, conv)
		s.webhooks.Emit(ctx, conv.TenantID, entity.EventConversationClosed, contracts.NewConversationPayload(conv))
		s.sla.OnResolved(ctx, conv, now)
		s.csat.OnConversationClosed(ctx, conv)
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
	if len(add) > 0 && s.tags != nil {
		if err := s.tags.ValidateTags(ctx, add); err != nil {
			return nil, err
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
	return conv, nil
}

// ListMessages returns the message timeline of a visible conversation.
func (s *Service) ListMessages(ctx context.Context, conversationID string, page shared.PageRequest) ([]*entity.Message, error) {
	if _, _, err := s.loadVisible(ctx, conversationID); err != nil {
		return nil, err
	}
	return s.messages.ListByConversation(ctx, conversationID, page.Normalize())
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

// MarkRead records that the actor read the conversation and publishes a
// message.read event to the conversation room.
func (s *Service) MarkRead(ctx context.Context, conversationID string) error {
	conv, ac, err := s.loadVisible(ctx, conversationID)
	if err != nil {
		return err
	}
	return s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID), contracts.RealtimeMessageRead,
		contracts.ReadPayload{ConversationID: conv.ID, UserID: ac.UserID, ReadAt: s.clock.Now()})
}

// ── internals ────────────────────────────────────────────────────────────────

// persistMessage stores a message, bumps conversation activity, records the
// timeline event, and publishes realtime message.created + conversation.updated.
func (s *Service) persistMessage(ctx context.Context, conv *entity.Conversation, msg *entity.Message, eventType string) (*entity.Message, error) {
	if err := s.messages.Create(ctx, msg); err != nil {
		return nil, err
	}

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

	// Outbound webhook: only real messages, not internal notes.
	if eventType == entity.EventMessageCreated {
		s.webhooks.Emit(ctx, conv.TenantID, entity.EventMessageCreated, contracts.NewMessagePayload(msg))
	}
	return msg, nil
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
