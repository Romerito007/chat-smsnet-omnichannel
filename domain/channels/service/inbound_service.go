package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	chrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
	contactcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/contracts"
	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	contactservice "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/service"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	groupentity "github.com/romerito007/chat-smsnet-omnichannel/domain/groups/entity"
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
	enricher      convcontracts.WebhookEnricher
	media         shared.IntegrationMediaResolver
	businessHours BusinessHoursChecker
	outOfHours    OutOfHoursSender
	groupGate     GroupGate
	logger        shared.Logger
}

// GroupGate resolves a synced WhatsApp group from the registry by its JID, so the
// inbound flow can decide whether to attend a group message. Implemented by the
// groups service (FindByJID). A not-found result means the group was never synced.
type GroupGate interface {
	FindByJID(ctx context.Context, groupJID string) (*groupentity.Group, error)
}

// SetGroupGate wires the group registry used to gate inbound group messages.
// Optional: without it, group messages (those carrying a group_jid) are discarded.
func (s *InboundService) SetGroupGate(g GroupGate) {
	if g != nil {
		s.groupGate = g
	}
}

// BusinessHoursChecker reports whether a channel is open at an instant (timezone +
// weekly schedule + holidays). Implemented by the businesshours service. A channel
// with no schedule is always open (24/7) → out-of-hours never fires.
type BusinessHoursChecker interface {
	IsWithinBusinessHours(ctx context.Context, channelID string, at time.Time) (bool, error)
}

// OutOfHoursSender sends the out-of-hours notice as an outbound message through the
// normal pipeline (webhook → gateway → customer). Implemented by the conversations
// service (SendAutomationMessage): SenderType=automation, so it is delivered but
// never re-triggers automation rules. ruleID is empty for the out-of-hours notice.
type OutOfHoursSender interface {
	SendAutomationMessage(ctx context.Context, conversationID, ruleID, text string) error
}

// SetBusinessHours wires the open/closed checker for the out-of-hours notice.
// Optional: without it (or without an OutOfHoursSender) the feature is off.
func (s *InboundService) SetBusinessHours(c BusinessHoursChecker) {
	if c != nil {
		s.businessHours = c
	}
}

// SetOutOfHoursSender wires the sender used to deliver the out-of-hours notice.
// Optional.
func (s *InboundService) SetOutOfHoursSender(o OutOfHoursSender) {
	if o != nil {
		s.outOfHours = o
	}
}

// SetLogger wires the structured logger used to record an inbound attachment
// storage failure with its routing context. Optional: defaults to slog.Default().
func (s *InboundService) SetLogger(l shared.Logger) {
	if l != nil {
		s.logger = l
	}
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

// SetWebhookEnricher wires the resolver of the outbound-webhook contact block.
// Inbound enriches the CONTACT only (a customer message has no agent), lazily —
// the contact is resolved only when a subscription matches the event. Optional.
func (s *InboundService) SetWebhookEnricher(e convcontracts.WebhookEnricher) {
	if e != nil {
		s.enricher = e
	}
}

// integrationContact best-effort resolves the recipient block for the webhook.
// nil-safe: nil when no enricher is wired or it can't resolve.
func (s *InboundService) integrationContact(ctx context.Context, contactID string) *convcontracts.WebhookContact {
	if s.enricher == nil || contactID == "" {
		return nil
	}
	return s.enricher.WebhookContact(ctx, contactID)
}

// emitConversationWebhook emits a conversation lifecycle webhook with the lazy
// integration payload — the recipient contact (no agent on inbound), resolved only
// when a subscription matches.
func (s *InboundService) emitConversationWebhook(ctx context.Context, conv *conventity.Conversation, event string) {
	s.webhooks.EmitLazy(ctx, conv.TenantID, event, conv.SectorID, func() any {
		return convcontracts.NewIntegrationConversationPayload(conv, s.integrationContact(ctx, conv.ContactID), nil)
	})
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
		logger:   slog.Default(),
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
	groupJID := strings.TrimSpace(msg.GroupJID)
	isGroup := groupJID != ""
	externalContact := strings.TrimSpace(msg.ExternalContactID)
	if externalContact == "" {
		externalContact = strings.TrimSpace(msg.ContactPhone)
	}
	// A 1:1 message must identify its sender; a group message is keyed by the group
	// JID instead (the sender is recorded as message metadata, not as the key).
	if !isGroup && externalContact == "" {
		return chcontracts.InboundResult{}, apperror.Validation("external_contact_id or contact_phone is required")
	}
	if strings.TrimSpace(msg.Text) == "" && len(msg.Attachments) == 0 && len(msg.RawAttachments) == 0 &&
		len(msg.Contacts) == 0 && msg.Location == nil && msg.InteractiveReply == nil {
		return chcontracts.InboundResult{}, apperror.Validation("text, attachments, contacts, location or interactive_reply are required")
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

	contact, discarded, err := s.resolveInboundContact(ctx, channel, groupJID, externalContact, msg)
	if err != nil {
		return chcontracts.InboundResult{}, err
	}
	if discarded {
		// Intentionally dropped (group not attended / not synced): 200, nothing stored.
		// Log loudly with the payload shape so a swallowed body is never invisible.
		shared.LoggerFrom(ctx, s.logger).Info("INBOUND_NO_MESSAGE_CREATED",
			"reason", "group_discarded", "channel", channel, "group_jid", groupJID,
			"external_message_id", externalMsgID,
			"has_text", strings.TrimSpace(msg.Text) != "",
			"has_contacts", len(msg.Contacts) > 0, "has_location", msg.Location != nil,
			"has_attachments", len(msg.Attachments) > 0 || len(msg.RawAttachments) > 0)
		return chcontracts.InboundResult{Discarded: true, Status: "discarded"}, nil
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

	// Out-of-hours notice: only on a NEW conversation, only when the channel has a
	// message configured AND is currently closed. Best-effort — it must never block
	// or fail the inbound (the conversation already entered normally).
	if isNew {
		s.maybeSendOutOfHours(ctx, conn, conv)
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

// resolveInboundContact maps an inbound message to the contact whose conversation it
// belongs to. For a 1:1 message that is the SENDER (created per person, as before).
// For a GROUP message (groupJID set) it is the single GROUP contact — gated by the
// registry: a group that was never synced, or whose attend flag is off, is DISCARDED
// (returns discarded=true; nothing is created). The group member is never a contact.
func (s *InboundService) resolveInboundContact(ctx context.Context, channel, groupJID, externalContact string, msg chcontracts.InboundMessage) (*contactentity.Contact, bool, error) {
	if groupJID == "" {
		contact, err := s.contacts.UpsertFromInbound(ctx, contactcontracts.UpsertFromInbound{
			Channel:    channel,
			ExternalID: externalContact,
			Name:       msg.ContactName,
			Phone:      msg.ContactPhone,
			Document:   msg.ContactDocument,
		})
		return contact, false, err
	}

	// Group message: it is only attended when the group was synced AND attend=true.
	if s.groupGate == nil {
		shared.LoggerFrom(ctx, s.logger).Warn("GROUP_GATE_NOT_CONFIGURED", "group_jid", groupJID)
		return nil, true, nil
	}
	grp, err := s.groupGate.FindByJID(ctx, groupJID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			shared.LoggerFrom(ctx, s.logger).Info("GROUP_NOT_SYNCED_DISCARDED", "group_jid", groupJID)
			return nil, true, nil // never synced → not attended
		}
		return nil, false, err
	}
	if !grp.Attend {
		shared.LoggerFrom(ctx, s.logger).Info("GROUP_NOT_ATTENDED_DISCARDED", "group_jid", groupJID, "group_id", grp.ID)
		return nil, true, nil
	}
	contact, err := s.contacts.UpsertGroupContact(ctx, channel, groupJID, grp.Name, grp.Description)
	return contact, false, err
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
	s.emitConversationWebhook(ctx, conv, convcontracts.RealtimeConversationUpdated)
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

// maybeSendOutOfHours sends the channel's out-of-hours notice to the customer when
// a NEW conversation opens outside business hours. Off when the channel has no
// message, or no checker/sender is wired. Best-effort: any error is logged and
// swallowed so the inbound flow is never affected.
func (s *InboundService) maybeSendOutOfHours(ctx context.Context, conn *chentity.ChannelConnection, conv *conventity.Conversation) {
	text := strings.TrimSpace(conn.OutOfHoursMessage)
	if text == "" || s.businessHours == nil || s.outOfHours == nil {
		return
	}
	open, err := s.businessHours.IsWithinBusinessHours(ctx, conn.ID, s.clock.Now())
	if err != nil {
		shared.LoggerFrom(ctx, s.logger).Warn("OUT_OF_HOURS_CHECK_FAILED", "channel_id", conn.ID, "conversation_id", conv.ID, "error", err.Error())
		return
	}
	if open {
		return // within business hours → no notice
	}
	if err := s.outOfHours.SendAutomationMessage(ctx, conv.ID, "", text); err != nil {
		shared.LoggerFrom(ctx, s.logger).Warn("OUT_OF_HOURS_SEND_FAILED", "channel_id", conn.ID, "conversation_id", conv.ID, "error", err.Error())
	}
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
			// Enrich the storage failure with the inbound routing context (the
			// attachments service already logged provider/key/cause) before it
			// surfaces as the 5xx the gateway sees.
			shared.LoggerFrom(ctx, s.logger).Error("inbound attachment storage failed",
				"channel_id", conv.ChannelID, "conversation_id", conv.ID,
				"external_message_id", in.ExternalMessageID, "field_name", "attachments[]",
				"file_count", len(in.RawAttachments), "filename", f.Filename,
				"content_type", f.ContentType, "size", len(f.Data), "cause", err.Error())
			return nil, err
		}
		shared.LoggerFrom(ctx, s.logger).Info("INBOUND_ATTACHMENT_STORED",
			"conversation_id", conv.ID, "attachment_id", att.ID, "filename", att.Filename,
			"content_type", att.ContentType, "size", att.Size)
		attachments = append(attachments, att)
	}

	// Pick the message type by payload, validating the structured ones so a malformed
	// inbound is a 4xx, not a corrupt stored message.
	mtype := conventity.MessageText
	text := in.Text
	var reply *conventity.InteractiveReply
	switch {
	case len(in.Contacts) > 0:
		if msg := conventity.ValidateContacts(in.Contacts); msg != "" {
			return nil, apperror.Validation(msg).WithDetails(map[string]any{"contacts": msg})
		}
		mtype = conventity.MessageContact
	case in.Location != nil:
		if msg := in.Location.Validate(); msg != "" {
			return nil, apperror.Validation(msg).WithDetails(map[string]any{"location": msg})
		}
		mtype = conventity.MessageLocation
	case in.InteractiveReply != nil:
		mtype = conventity.MessageInteractiveReply
		reply = s.resolveInteractiveReply(ctx, conv.ID, in.InteractiveReply)
		if strings.TrimSpace(text) == "" {
			text = reply.Title // mirror the chosen title so search/automation keep working
		}
	case len(attachments) > 0:
		// Derive the renderable media type from the first attachment so an inbound
		// image/audio/video is reported as such on BOTH rails (REST read + realtime),
		// even when it carries a caption — never collapsed to "text" or "file".
		mtype = conventity.MessageTypeForContentType(attachments[0].ContentType)
	}

	message := &conventity.Message{
		ID:                shared.NewID(),
		TenantID:          conv.TenantID,
		ConversationID:    conv.ID,
		SenderType:        conventity.SenderCustomer,
		SenderID:          contactID,
		Direction:         conventity.DirectionInbound,
		MessageType:       mtype,
		Text:              text,
		Attachments:       attachments,
		Contacts:          in.Contacts,
		Location:          in.Location,
		InteractiveReply:  reply,
		GroupSender:       groupSenderFrom(in),
		Metadata:          in.Metadata,
		CreatedAt:         createdAt,
		DeliveryStatus:    conventity.DeliveryNone,
		ExternalMessageID: strings.TrimSpace(in.ExternalMessageID),
	}
	if err := s.messages.Create(ctx, message); err != nil {
		return nil, err
	}
	if len(attachments) > 0 {
		shared.LoggerFrom(ctx, s.logger).Info("INBOUND_MESSAGE_CREATED_WITH_ATTACHMENTS",
			"message_id", message.ID, "attachment_count", len(attachments),
			"message_type", string(mtype), "direction", string(conventity.DirectionInbound))
	}

	conv.LastMessageAt = createdAt
	// Track the last INBOUND (customer) message for the WhatsApp 24h service window.
	inboundAt := createdAt
	conv.LastCustomerMessageAt = &inboundAt
	// Refresh the denormalized last-message snapshot the inbox reads (no aggregation).
	conv.LastMessage = conventity.NewLastMessageSnapshot(message)
	conv.UpdatedAt = createdAt
	// A new customer message increments the unread counter for agents; reset by
	// MarkRead (POST /read).
	conv.UnreadCount++
	if err := s.conversations.Update(ctx, conv); err != nil {
		return nil, err
	}

	s.recordEvent(ctx, conv, conventity.EventMessageCreated, map[string]any{"message_id": message.ID})
	realtimePayload := convcontracts.NewMessagePayload(message)
	if len(realtimePayload.Attachments) > 0 {
		shared.LoggerFrom(ctx, s.logger).Info("REALTIME_MESSAGE_CREATED_PAYLOAD",
			"message_id", message.ID, "attachment_count", len(realtimePayload.Attachments),
			"message_type", realtimePayload.MessageType)
	}
	// Realtime: message.created + conversation.updated (conversation topic, and the
	// sector inbox topic) in ONE Redis round trip, ordered (message before update).
	convPayload := convcontracts.NewConversationPayload(conv)
	events := []shared.PublishEvent{
		{Topic: shared.TopicConversation(conv.TenantID, conv.ID), Event: convcontracts.RealtimeMessageCreated, Data: realtimePayload},
		{Topic: shared.TopicConversation(conv.TenantID, conv.ID), Event: convcontracts.RealtimeConversationUpdated, Data: convPayload},
	}
	// Inbox rooms: the sector room, or — for a queued/sector-less conversation — the
	// unassigned room (+ assignee), so the agent's inbox updates live for new inbound
	// messages on unassigned conversations too (mirrors the REST inbox visibility).
	for _, topic := range shared.InboxTopicsFor(conv.TenantID, conv.SectorID, conv.AssignedTo) {
		events = append(events, shared.PublishEvent{Topic: topic, Event: convcontracts.RealtimeConversationUpdated, Data: convPayload})
	}
	shared.PublishAll(ctx, s.publisher, events...)
	// Automation: a new INBOUND customer message must trigger message_created rules
	// (e.g. "message content contains X" → assign team). The payload is the MESSAGE
	// (carrying its text), not the conversation, so the message_content condition can
	// match. Customer messages are never automation-authored, so there is no loop to
	// guard against here.
	if s.ruleSink != nil {
		s.ruleSink.EmitRuleEvent(ctx, conv.TenantID, conventity.EventMessageCreated, conv.ID, realtimePayload)
	}
	// Inbound customer messages flow to webhooks too (Chatwoot model: entrada and
	// saída both via message_created), with signed channel-media attachment URLs.
	s.webhooks.EmitLazy(ctx, conv.TenantID, conventity.EventMessageCreated, conv.SectorID, func() any {
		p := convcontracts.NewIntegrationMessagePayload(message, s.integrationMedia(ctx, message))
		p.Contact = s.integrationContact(ctx, conv.ContactID)
		p.Conversation = &convcontracts.WebhookConversationRef{CustomAttributes: conv.CustomAttributes}
		return p
	})
	return message, nil
}

// groupSenderFrom builds the group-sender metadata for an inbound message — only on
// a group message (group_jid set) that carries at least one sender field. The member
// is recorded for display, never as a contact. Nil on 1:1 messages.
func groupSenderFrom(in chcontracts.InboundMessage) *conventity.GroupSender {
	if strings.TrimSpace(in.GroupJID) == "" {
		return nil
	}
	jid := strings.TrimSpace(in.SenderJID)
	name := strings.TrimSpace(in.SenderName)
	phone := strings.TrimSpace(in.SenderPhone)
	if jid == "" && name == "" && phone == "" {
		return nil
	}
	return &conventity.GroupSender{JID: jid, Name: name, Phone: phone}
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
	s.emitConversationWebhook(ctx, conv, conventity.EventConversationCreated)
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

// resolveInteractiveReply builds the stored interactive reply, resolving the menu's
// external context id (Meta context.id) to the INTERNAL id of the menu message we
// sent — best-effort: if the menu isn't found (e.g. its external id wasn't recorded
// yet), ContextMessageID is left empty rather than failing the inbound.
func (s *InboundService) resolveInteractiveReply(ctx context.Context, conversationID string, in *chcontracts.InboundInteractiveReply) *conventity.InteractiveReply {
	r := &conventity.InteractiveReply{Type: in.Type, ID: in.ID, Title: in.Title, Description: in.Description}
	ext := strings.TrimSpace(in.ContextExternalID)
	if ext != "" {
		if menu, err := s.messages.FindByExternalMessageID(ctx, conversationID, ext); err == nil && menu != nil {
			r.ContextMessageID = menu.ID
		}
	}
	return r
}

func (s *InboundService) publishConversation(ctx context.Context, conv *conventity.Conversation) {
	payload := convcontracts.NewConversationPayload(conv)
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID),
		convcontracts.RealtimeConversationUpdated, payload)
	// Inbox rooms mirroring the REST visibility: sector room, or the unassigned room
	// (+ assignee) for a queued/sector-less conversation.
	for _, topic := range shared.InboxTopicsFor(conv.TenantID, conv.SectorID, conv.AssignedTo) {
		_ = s.publisher.Publish(ctx, topic, convcontracts.RealtimeConversationUpdated, payload)
	}
}
