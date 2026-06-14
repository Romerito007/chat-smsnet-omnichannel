package service

import (
	"context"
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
// (automation or enqueue) and realtime — all fast; slow work is deferred to
// Asynq.
type InboundService struct {
	contacts      *contactservice.Service
	conversations convrepo.ConversationRepository
	messages      convrepo.MessageRepository
	events        convrepo.EventRepository
	inbound       chrepo.InboundRepository
	dispatcher    chcontracts.AutomationDispatcher
	locker        shared.Locker
	publisher     shared.EventPublisher
	clock         shared.Clock
	attachments   chcontracts.InboundAttachmentStore
}

// SetAttachmentStore wires the persister for raw (multipart) inbound attachments.
// Optional: when unset, only URL-mode attachments are accepted.
func (s *InboundService) SetAttachmentStore(a chcontracts.InboundAttachmentStore) {
	if a != nil {
		s.attachments = a
	}
}

// NewInboundService builds the orchestrator.
func NewInboundService(
	contacts *contactservice.Service,
	conversations convrepo.ConversationRepository,
	messages convrepo.MessageRepository,
	events convrepo.EventRepository,
	inbound chrepo.InboundRepository,
	dispatcher chcontracts.AutomationDispatcher,
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
		events: events, inbound: inbound, dispatcher: dispatcher,
		locker: locker, publisher: publisher, clock: clock,
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

	conv, isNew, err := s.findOrCreateConversation(ctx, tenantID, contact.ID, channel, conn.ID)
	if err != nil {
		return chcontracts.InboundResult{}, err
	}

	message, err := s.appendInboundMessage(ctx, conv, contact.ID, msg)
	if err != nil {
		return chcontracts.InboundResult{}, err
	}

	// New conversations are routed: automation first, else enqueue.
	if isNew {
		if err := s.routeNew(ctx, conn, conv, message.ID); err != nil {
			return chcontracts.InboundResult{}, err
		}
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

func (s *InboundService) findOrCreateConversation(ctx context.Context, tenantID, contactID, channel, channelID string) (*conventity.Conversation, bool, error) {
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
	conv = &conventity.Conversation{
		ID:            shared.NewID(),
		TenantID:      tenantID,
		ContactID:     contactID,
		Channel:       channel,
		ChannelID:     channelID,
		Status:        conventity.StatusNew,
		Priority:      conventity.PriorityNormal,
		LastMessageAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.conversations.Create(ctx, conv); err != nil {
		return nil, false, err
	}
	s.recordEvent(ctx, conv, conventity.EventConversationCreated, nil)
	return conv, true, nil
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
	return message, nil
}

// routeNew applies the initial routing of a brand-new conversation: automation
// (slow work deferred to Asynq) when enabled, otherwise enqueue into the
// connection's default sector.
func (s *InboundService) routeNew(ctx context.Context, conn *chentity.ChannelConnection, conv *conventity.Conversation, messageID string) error {
	if conn.AutomationEnabled {
		conv.Status = conventity.StatusAutomation
		conv.UpdatedAt = s.clock.Now()
		if err := s.conversations.Update(ctx, conv); err != nil {
			return err
		}
		s.recordEvent(ctx, conv, conventity.EventConversationUpdated, map[string]any{"automation": true})
		if s.dispatcher != nil {
			_ = s.dispatcher.Dispatch(chcontracts.AutomationInvoke{
				TenantID:       conv.TenantID,
				IntegrationID:  conn.ID,
				ConversationID: conv.ID,
				MessageID:      messageID,
			})
		}
		s.publishConversation(ctx, conv)
		return nil
	}

	// No automation: enqueue into the connection's default sector.
	conv.SectorID = conn.DefaultSectorID
	conv.Status = conventity.StatusQueued
	conv.UpdatedAt = s.clock.Now()
	if err := s.conversations.Update(ctx, conv); err != nil {
		return err
	}
	s.recordEvent(ctx, conv, conventity.EventConversationEnqueued, map[string]any{"sector_id": conv.SectorID})
	s.publishConversation(ctx, conv)
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
