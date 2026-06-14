package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	chrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
	contactcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/contracts"
	contactservice "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/service"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// inboundLockTTL bounds the per-message processing lock.
const inboundLockTTL = 10 * time.Second

// InboundService orchestrates an inbound channel message: idempotency, contact
// upsert, conversation find/create, message persistence, initial routing
// (enqueue into the connection's default sector) and realtime — all fast; slow
// work is deferred to Asynq.
type InboundService struct {
	contacts      *contactservice.Service
	conversations convrepo.ConversationRepository
	messages      convrepo.MessageRepository
	events        convrepo.EventRepository
	protocols     convrepo.ProtocolCounterRepository
	inbound       chrepo.InboundRepository
	locker        shared.Locker
	publisher     shared.EventPublisher
	clock         shared.Clock
	attachments   chcontracts.InboundAttachmentStore
	ruleSink      shared.RuleEventSink
	webhooks      shared.WebhookEmitter
	media         shared.IntegrationMediaResolver
}

// SetAttachmentStore wires the persister for raw (multipart) inbound attachments.
// Optional: when unset, only URL-mode attachments are accepted.
func (s *InboundService) SetAttachmentStore(a chcontracts.InboundAttachmentStore) {
	if a != nil {
		s.attachments = a
	}
}

// SetRuleSink wires the automation-rules event sink. Optional: when unset, inbound
// lifecycle transitions (conversation created/reopened) emit no rule events.
func (s *InboundService) SetRuleSink(sink shared.RuleEventSink) {
	if sink != nil {
		s.ruleSink = sink
	}
}

// SetWebhookEmitter wires the outbound webhook emitter so inbound customer
// messages and conversation lifecycle (created/reopened) flow to the tenant's
// webhooks — the same pipeline an agent's outbound message uses. Optional.
func (s *InboundService) SetWebhookEmitter(e shared.WebhookEmitter) {
	if e != nil {
		s.webhooks = e
	}
}

// SetIntegrationMediaResolver wires the resolver that turns inbound attachment ids
// into signed, public channel-media URLs for the webhook payload. Optional.
func (s *InboundService) SetIntegrationMediaResolver(r shared.IntegrationMediaResolver) {
	if r != nil {
		s.media = r
	}
}

// NewInboundService builds the orchestrator.
func NewInboundService(
	contacts *contactservice.Service,
	conversations convrepo.ConversationRepository,
	messages convrepo.MessageRepository,
	events convrepo.EventRepository,
	protocols convrepo.ProtocolCounterRepository,
	inbound chrepo.InboundRepository,
	locker shared.Locker,
	publisher shared.EventPublisher,
	clock shared.Clock,
) *InboundService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if publisher == nil {
		publisher = shared.NoopPublisher{}
	}
	if locker == nil {
		locker = shared.NoopLocker{}
	}
	return &InboundService{
		contacts: contacts, conversations: conversations, messages: messages,
		events: events, protocols: protocols, inbound: inbound,
		locker: locker, publisher: publisher, clock: clock,
		webhooks: shared.NoopWebhookEmitter{},
	}
}

// Handle processes one inbound message. The context must be tenant-scoped (set
// from the authenticated connection).
func (s *InboundService) Handle(ctx context.Context, conn *chentity.ChannelConnection, msg chcontracts.InboundMessage) (chcontracts.InboundResult, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return chcontracts.InboundResult{}, err
	}
	channel := string(conn.Type)

	externalMsgID := strings.TrimSpace(msg.ExternalMessageID)
	if externalMsgID == "" {
		return chcontracts.InboundResult{}, apperror.Validation("external_message_id is required")
	}
	externalContact := strings.TrimSpace(msg.ExternalContactID)
	if externalContact == "" {
		externalContact = strings.TrimSpace(msg.ContactPhone)
	}
	if externalContact == "" {
		return chcontracts.InboundResult{}, apperror.Validation("external_contact_id or contact_phone is required")
	}
	if strings.TrimSpace(msg.Text) == "" && len(msg.Attachments) == 0 && len(msg.RawAttachments) == 0 {
		return chcontracts.InboundResult{}, apperror.Validation("text or attachments are required")
	}
	if len(msg.RawAttachments) > 0 && s.attachments == nil {
		return chcontracts.InboundResult{}, apperror.Internal("attachment storage is not configured")
	}

	// Serialize processing of the same external message across nodes.
	key := "inbound:lock:" + tenantID + ":" + channel + ":" + externalMsgID
	release, acquired, err := s.locker.Acquire(ctx, key, inboundLockTTL)
	if err != nil {
		return chcontracts.InboundResult{}, apperror.Internal("could not acquire inbound lock").Wrap(err)
	}
	if !acquired {
		return chcontracts.InboundResult{}, apperror.Conflict("inbound message is already being processed")
	}
	defer release()

	// Idempotency: a previously processed external id short-circuits.
	if existing, err := s.inbound.FindByExternalID(ctx, channel, externalMsgID); err == nil {
		return s.idempotentResult(ctx, existing), nil
	} else if apperror.From(err).Code != apperror.CodeNotFound {
		return chcontracts.InboundResult{}, err
	}

	contact, err := s.contacts.UpsertFromInbound(ctx, contactcontracts.UpsertFromInbound{
		Channel:    channel,
		ExternalID: externalContact,
		Name:       msg.ContactName,
		Phone:      msg.ContactPhone,
		Document:   msg.ContactDocument,
	})
	if err != nil {
		return chcontracts.InboundResult{}, err
	}

	conv, isNew, err := s.findOrCreateConversation(ctx, tenantID, contact.ID, channel, conn.ID, conn.UsesProtocol)
	if err != nil {
		return chcontracts.InboundResult{}, err
	}

	// New conversations enter queued (no sector) awaiting assignment; route first so
	// the conversation_created webhook reflects the final (queued) state.
	if isNew {
		if err := s.routeNew(ctx, conv); err != nil {
			return chcontracts.InboundResult{}, err
		}
	}

	message, err := s.appendInboundMessage(ctx, conv, contact.ID, msg)
	if err != nil {
		return chcontracts.InboundResult{}, err
	}

	// Record idempotency last; a duplicate here (race despite the lock) is benign.
	rec := &chentity.InboundRecord{
		ID:                shared.NewID(),
		TenantID:          tenantID,
		Channel:           channel,
		ExternalMessageID: externalMsgID,
		ConversationID:    conv.ID,
		MessageID:         message.ID,
		CreatedAt:         s.clock.Now(),
	}
	if err := s.inbound.Create(ctx, rec); err != nil && apperror.From(err).Code != apperror.CodeConflict {
		return chcontracts.InboundResult{}, err
	}

	return chcontracts.InboundResult{
		ConversationID: conv.ID,
		MessageID:      message.ID,
		ContactID:      contact.ID,
		Status:         string(conv.Status),
		Idempotent:     false,
	}, nil
}

func (s *InboundService) idempotentResult(ctx context.Context, rec *chentity.InboundRecord) chcontracts.InboundResult {
	status := ""
	if conv, err := s.conversations.FindByID(ctx, rec.ConversationID); err == nil {
		status = string(conv.Status)
	}
	return chcontracts.InboundResult{
		ConversationID: rec.ConversationID,
		MessageID:      rec.MessageID,
		Status:         status,
		Idempotent:     true,
	}
}

// findOrCreateConversation resolves the conversation an inbound message belongs
// to. The "open conversation exists" case is identical in both modes (reuse it,
// keeping its protocol if any). The modes differ only when there is NO open
// conversation:
//
//   - single mode (usesProtocol=false): REOPEN the contact's last conversation on
//     this channel (any status) if one exists; otherwise create the first one. No
//     protocol is assigned.
//   - protocol mode (usesProtocol=true): always create a NEW conversation with a
//     NEW protocol number (a closed last one is not reopened).
//
// The bool return is true only when a brand-new conversation was created (so the
// caller routes it); a reused or reopened conversation returns false.
func (s *InboundService) findOrCreateConversation(ctx context.Context, tenantID, contactID, channel, channelID string, usesProtocol bool) (*conventity.Conversation, bool, error) {
	// Reuse an open conversation for this contact on the SAME channel connection
	// (not just the same type) — two connections of the same type are distinct.
	conv, err := s.conversations.FindOpenByContactChannelID(ctx, contactID, channelID)
	if err == nil {
		return conv, false, nil
	}
	if apperror.From(err).Code != apperror.CodeNotFound {
		return nil, false, err
	}

	now := s.clock.Now()

	// Single mode: reopen the contact's last conversation on this channel (closed,
	// since no open one was found) instead of creating a new one.
	if !usesProtocol {
		last, lerr := s.conversations.FindLastByContactChannelID(ctx, contactID, channelID)
		if lerr == nil {
			if err := s.reopen(ctx, last, now); err != nil {
				return nil, false, err
			}
			return last, false, nil
		}
		if apperror.From(lerr).Code != apperror.CodeNotFound {
			return nil, false, lerr
		}
		// Never had a conversation here → fall through to create the first one.
	}

	// Protocol mode assigns a fresh per-tenant/year protocol number; single mode
	// leaves it empty.
	protocol := ""
	if usesProtocol {
		p, perr := s.nextProtocol(ctx, tenantID, now)
		if perr != nil {
			return nil, false, perr
		}
		protocol = p
	}

	conv = &conventity.Conversation{
		ID:            shared.NewID(),
		TenantID:      tenantID,
		ContactID:     contactID,
		Channel:       channel,
		ChannelID:     channelID,
		Status:        conventity.StatusNew,
		Priority:      conventity.PriorityNormal,
		Protocol:      protocol,
		LastMessageAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.conversations.Create(ctx, conv); err != nil {
		return nil, false, err
	}
	s.recordEvent(ctx, conv, conventity.EventConversationCreated, nil)
	s.emitRule(ctx, conv, conventity.EventConversationCreated)
	return conv, true, nil
}

// reopen revives a closed conversation in place: status returns to new (or
// assigned/queued if it still has an agent/queue), closed_at is cleared. The
// assignee, sector, queue, tags and protocol are KEPT — the same agent picks it
// back up. Records the reopened timeline event and emits the rule event
// (conversation.reopened → automation-rules conversation_opened). Realtime publish
// happens on the inbound message append that follows.
func (s *InboundService) reopen(ctx context.Context, conv *conventity.Conversation, now time.Time) error {
	if !conv.Status.IsClosed() {
		return nil // defensive: an open one would have been returned by the reuse path
	}
	conv.Status = conventity.StatusNew
	if conv.AssignedTo != "" {
		conv.Status = conventity.StatusAssigned
	} else if conv.QueueID != "" {
		conv.Status = conventity.StatusQueued
	}
	conv.ClosedAt = nil
	conv.UpdatedAt = now
	if err := s.conversations.Update(ctx, conv); err != nil {
		return err
	}
	s.recordEvent(ctx, conv, conventity.EventConversationReopened, nil)
	s.emitRule(ctx, conv, conventity.EventConversationReopened)
	// A reopen is a significant change → conversation_updated webhook.
	s.webhooks.Emit(ctx, conv.TenantID, convcontracts.RealtimeConversationUpdated, conv.SectorID, convcontracts.NewConversationPayload(conv))
	return nil
}

// nextProtocol formats the next per-tenant/year protocol number as "2026-000123".
// The sequence comes from an atomic counter (no count-and-add race). The year is
// taken in UTC.
func (s *InboundService) nextProtocol(ctx context.Context, tenantID string, now time.Time) (string, error) {
	year := now.UTC().Year()
	seq, err := s.protocols.NextSequence(ctx, tenantID, year)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d-%06d", year, seq), nil
}

// emitRule forwards an inbound lifecycle event to the automation-rules sink (best
// effort; no-op when the sink is unset).
func (s *InboundService) emitRule(ctx context.Context, conv *conventity.Conversation, event string) {
	if s.ruleSink != nil {
		s.ruleSink.EmitRuleEvent(ctx, conv.TenantID, event, conv.ID, convcontracts.NewConversationPayload(conv))
	}
}

func (s *InboundService) appendInboundMessage(ctx context.Context, conv *conventity.Conversation, contactID string, in chcontracts.InboundMessage) (*conventity.Message, error) {
	createdAt := s.clock.Now()
	if in.Timestamp > 0 {
		createdAt = time.UnixMilli(in.Timestamp).UTC()
	}

	// Persist any raw (multipart) attachments now that the conversation exists, so
	// each record is access-checked on download; merge with URL-mode attachments.
	attachments := append([]conventity.Attachment(nil), in.Attachments...)
	for _, f := range in.RawAttachments {
		att, err := s.attachments.StoreInbound(ctx, conv.ID, f.Filename, f.ContentType, f.Data)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, att)
	}

	mtype := conventity.MessageText
	if strings.TrimSpace(in.Text) == "" && len(attachments) > 0 {
		mtype = conventity.MessageFile
	}

	message := &conventity.Message{
		ID:                shared.NewID(),
		TenantID:          conv.TenantID,
		ConversationID:    conv.ID,
		SenderType:        conventity.SenderCustomer,
		SenderID:          contactID,
		Direction:         conventity.DirectionInbound,
		MessageType:       mtype,
		Text:              in.Text,
		Attachments:       attachments,
		Metadata:          in.Metadata,
		CreatedAt:         createdAt,
		DeliveryStatus:    conventity.DeliveryNone,
		ExternalMessageID: strings.TrimSpace(in.ExternalMessageID),
	}
	if err := s.messages.Create(ctx, message); err != nil {
		return nil, err
	}

	conv.LastMessageAt = createdAt
	conv.UpdatedAt = createdAt
	// A new customer message increments the unread counter for agents; reset by
	// MarkRead (POST /read).
	conv.UnreadCount++
	if err := s.conversations.Update(ctx, conv); err != nil {
		return nil, err
	}

	s.recordEvent(ctx, conv, conventity.EventMessageCreated, map[string]any{"message_id": message.ID})
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID),
		convcontracts.RealtimeMessageCreated, convcontracts.NewMessagePayload(message))
	s.publishConversation(ctx, conv)
	// Inbound customer messages flow to webhooks too (Chatwoot model: entrada and
	// saída both via message_created), with signed channel-media attachment URLs.
	s.webhooks.Emit(ctx, conv.TenantID, conventity.EventMessageCreated, conv.SectorID,
		convcontracts.NewIntegrationMessagePayload(message, s.integrationMedia(ctx, message)))
	return message, nil
}

// integrationMedia best-effort resolves a message's attachment ids to signed,
// public channel-media URLs (keyed by id) for the outbound webhook payload.
func (s *InboundService) integrationMedia(ctx context.Context, msg *conventity.Message) map[string]string {
	if s.media == nil || len(msg.Attachments) == 0 {
		return nil
	}
	out := make(map[string]string, len(msg.Attachments))
	for _, a := range msg.Attachments {
		if a.ID == "" {
			continue
		}
		if u, err := s.media.IntegrationMediaURL(ctx, a.ID); err == nil && u != "" {
			out[a.ID] = u
		}
	}
	return out
}

// routeNew applies the initial routing of a brand-new conversation. The channel
// no longer carries a sector (Chatwoot model: the team/sector is decided on the
// conversation, not the inbox), so a new conversation enters queued WITHOUT a
// sector, awaiting assignment by an agent or an automation rule.
func (s *InboundService) routeNew(ctx context.Context, conv *conventity.Conversation) error {
	conv.Status = conventity.StatusQueued
	conv.UpdatedAt = s.clock.Now()
	if err := s.conversations.Update(ctx, conv); err != nil {
		return err
	}
	s.recordEvent(ctx, conv, conventity.EventConversationEnqueued, nil)
	s.publishConversation(ctx, conv)
	// The conversation is now in its initial (queued) state → fan out to webhooks.
	s.webhooks.Emit(ctx, conv.TenantID, conventity.EventConversationCreated, conv.SectorID, convcontracts.NewConversationPayload(conv))
	return nil
}

func (s *InboundService) recordEvent(ctx context.Context, conv *conventity.Conversation, eventType string, data map[string]any) {
	_ = s.events.Create(ctx, &conventity.ConversationEvent{
		ID:             shared.NewID(),
		TenantID:       conv.TenantID,
		ConversationID: conv.ID,
		Type:           eventType,
		ActorType:      conventity.ActorSystem,
		Data:           data,
		CreatedAt:      s.clock.Now(),
	})
}

func (s *InboundService) publishConversation(ctx context.Context, conv *conventity.Conversation) {
	payload := convcontracts.NewConversationPayload(conv)
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID),
		convcontracts.RealtimeConversationUpdated, payload)
	if conv.SectorID != "" {
		_ = s.publisher.Publish(ctx, shared.TopicInbox(conv.TenantID, conv.SectorID),
			convcontracts.RealtimeConversationUpdated, payload)
	}
}
