package service

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	chrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/repository"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// defaultMaxAttempts bounds delivery retries before marking failed.
const defaultMaxAttempts = 5

// OutboundService delivers outbound messages to channels (via adapters), drives
// retries, and applies delivery receipts. It implements the conversations
// OutboundDispatcher port.
type OutboundService struct {
	connections   chrepo.ConnectionRepository
	deliveries    chrepo.OutboundDeliveryRepository
	conversations convrepo.ConversationRepository
	messages      convrepo.MessageRepository
	contacts      contactrepo.ContactRepository
	registry      chcontracts.AdapterRegistry
	enqueuer      chcontracts.DeliveryEnqueuer
	publisher     shared.EventPublisher
	clock         shared.Clock
	maxAttempts   int
	notifier      shared.Notifier
}

// SetNotifier wires the user notifier. Optional: when unset, channel delivery
// failures are not notified.
func (s *OutboundService) SetNotifier(n shared.Notifier) {
	if n != nil {
		s.notifier = n
	}
}

// NewOutboundService builds the service.
func NewOutboundService(
	connections chrepo.ConnectionRepository,
	deliveries chrepo.OutboundDeliveryRepository,
	conversations convrepo.ConversationRepository,
	messages convrepo.MessageRepository,
	contacts contactrepo.ContactRepository,
	registry chcontracts.AdapterRegistry,
	enqueuer chcontracts.DeliveryEnqueuer,
	publisher shared.EventPublisher,
	clock shared.Clock,
) *OutboundService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if publisher == nil {
		publisher = shared.NoopPublisher{}
	}
	return &OutboundService{
		connections: connections, deliveries: deliveries, conversations: conversations,
		messages: messages, contacts: contacts, registry: registry, enqueuer: enqueuer,
		publisher: publisher, clock: clock, maxAttempts: defaultMaxAttempts,
		notifier: shared.NoopNotifier{},
	}
}

// Dispatch (OutboundDispatcher) creates a delivery record for a pending outbound
// message and enqueues the channel.deliver job. Best-effort: any failure is
// swallowed so the agent's send is never blocked.
func (s *OutboundService) Dispatch(ctx context.Context, conv *conventity.Conversation, msg *conventity.Message) {
	if msg.Direction != conventity.DirectionOutbound {
		return
	}
	conn, err := s.connections.FindEnabledByType(ctx, chentity.Type(conv.Channel))
	if err != nil {
		return // no connection for this channel → leave the message pending
	}
	now := s.clock.Now()
	delivery := &chentity.OutboundDelivery{
		ID:                  shared.NewID(),
		TenantID:            conv.TenantID,
		ChannelConnectionID: conn.ID,
		ConversationID:      conv.ID,
		MessageID:           msg.ID,
		Status:              chentity.DeliveryPending,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := s.deliveries.Create(ctx, delivery); err != nil {
		return
	}
	if s.enqueuer != nil {
		_ = s.enqueuer.EnqueueDeliver(chcontracts.DeliverTask{TenantID: conv.TenantID, DeliveryID: delivery.ID})
	}
}

// Deliver attempts to send a delivery via its channel adapter. On success it
// marks the message sent; on failure it retries with backoff up to maxAttempts,
// then marks the message failed and publishes message.failed. It is idempotent:
// an already-sent delivery is a no-op. Called by the channel.deliver/retry jobs.
func (s *OutboundService) Deliver(ctx context.Context, deliveryID string) error {
	delivery, err := s.deliveries.FindByID(ctx, deliveryID)
	if err != nil {
		return err
	}
	if delivery.Status != chentity.DeliveryPending {
		return nil // already sent/delivered/failed → idempotent no-op
	}

	conn, err := s.connections.FindByID(ctx, delivery.ChannelConnectionID)
	if err != nil {
		return err
	}
	message, err := s.messages.FindByID(ctx, delivery.MessageID)
	if err != nil {
		return err
	}
	conv, err := s.conversations.FindByID(ctx, delivery.ConversationID)
	if err != nil {
		return err
	}

	adapter := s.registry.For(conn.Type)
	if adapter == nil {
		return s.fail(ctx, delivery, message, conv, "no adapter for channel type")
	}

	recipient := s.recipient(ctx, conv, conn.Type)
	res, err := adapter.SendMessage(ctx, conn, chcontracts.OutboundSend{
		DeliveryID:        delivery.ID,
		ConversationID:    conv.ID,
		ExternalContactID: recipient.ExternalID,
		Contact:           recipient,
		Text:              message.Text,
		Attachments:       message.Attachments,
		Metadata:          message.Metadata,
	})
	if err != nil {
		return s.retryOrFail(ctx, delivery, message, conv, err.Error())
	}

	now := s.clock.Now()
	delivery.Status = chentity.DeliverySent
	delivery.ExternalMessageID = res.ExternalMessageID
	delivery.Attempts++
	delivery.UpdatedAt = now
	if err := s.deliveries.Update(ctx, delivery); err != nil {
		return err
	}

	message.DeliveryStatus = conventity.DeliverySent
	message.ExternalMessageID = res.ExternalMessageID
	if err := s.messages.Update(ctx, message); err != nil {
		return err
	}
	s.publishStatus(ctx, conv, message, convcontracts.RealtimeMessageSent, "")
	return nil
}

// retryOrFail increments attempts and either schedules a retry with backoff or
// marks the delivery failed when the limit is reached.
func (s *OutboundService) retryOrFail(ctx context.Context, delivery *chentity.OutboundDelivery, message *conventity.Message, conv *conventity.Conversation, errMsg string) error {
	delivery.Attempts++
	delivery.LastError = errMsg
	if delivery.Attempts >= s.maxAttempts {
		return s.fail(ctx, delivery, message, conv, errMsg)
	}

	backoff := backoffSeconds(delivery.Attempts)
	next := s.clock.Now().Add(durationSeconds(backoff))
	delivery.NextRetryAt = &next
	delivery.UpdatedAt = s.clock.Now()
	if err := s.deliveries.Update(ctx, delivery); err != nil {
		return err
	}
	if s.enqueuer != nil {
		_ = s.enqueuer.EnqueueRetry(chcontracts.DeliverTask{TenantID: delivery.TenantID, DeliveryID: delivery.ID}, backoff)
	}
	return nil
}

// fail marks the delivery and message failed and publishes message.failed.
func (s *OutboundService) fail(ctx context.Context, delivery *chentity.OutboundDelivery, message *conventity.Message, conv *conventity.Conversation, errMsg string) error {
	now := s.clock.Now()
	delivery.Status = chentity.DeliveryFailed
	delivery.LastError = errMsg
	delivery.UpdatedAt = now
	if err := s.deliveries.Update(ctx, delivery); err != nil {
		return err
	}
	message.DeliveryStatus = conventity.DeliveryFailed
	message.DeliveryError = errMsg
	if err := s.messages.Update(ctx, message); err != nil {
		return err
	}
	s.publishStatus(ctx, conv, message, convcontracts.RealtimeMessageFailed, errMsg)
	// Notify the assigned agent that the channel could not deliver.
	if conv.AssignedTo != "" {
		s.notifier.Notify(ctx, shared.NotifyInput{
			TenantID: conv.TenantID, UserID: conv.AssignedTo,
			Type:  "channel.connection_error",
			Title: "A message could not be delivered to the channel",
			Link:  "/conversations/" + conv.ID,
		})
	}
	return nil
}

// ProcessReceipts parses a delivery-receipt payload via the channel adapter and
// applies each receipt, returning how many advanced a status.
func (s *OutboundService) ProcessReceipts(ctx context.Context, conn *chentity.ChannelConnection, rawBody []byte) (int, error) {
	adapter := s.registry.For(conn.Type)
	if adapter == nil {
		return 0, apperror.Integration("no adapter for channel type")
	}
	receipts, err := adapter.ParseDeliveryReceipt(rawBody)
	if err != nil {
		return 0, apperror.Validation("could not parse delivery receipts").Wrap(err)
	}
	applied := 0
	for _, rec := range receipts {
		if err := s.ApplyReceipt(ctx, rec); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}

// ApplyReceipt advances a delivery/message status from a channel receipt. It is
// idempotent: a receipt that does not advance the status is a no-op.
func (s *OutboundService) ApplyReceipt(ctx context.Context, receipt chcontracts.DeliveryReceipt) error {
	if receipt.ExternalMessageID == "" {
		return nil
	}
	delivery, err := s.deliveries.FindByExternalMessageID(ctx, receipt.ExternalMessageID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return nil // unknown external id → not ours
		}
		return err
	}
	if !chentity.Advances(delivery.Status, receipt.Status) {
		return nil // duplicate / out-of-order → idempotent no-op
	}

	now := s.clock.Now()
	delivery.Status = receipt.Status
	if receipt.Status == chentity.DeliveryFailed {
		delivery.LastError = receipt.Error
	}
	delivery.UpdatedAt = now
	if err := s.deliveries.Update(ctx, delivery); err != nil {
		return err
	}

	message, err := s.messages.FindByID(ctx, delivery.MessageID)
	if err != nil {
		return err
	}
	conv, _ := s.conversations.FindByID(ctx, delivery.ConversationID)

	event := ""
	switch receipt.Status {
	case chentity.DeliveryDelivered:
		message.DeliveryStatus = conventity.DeliveryDelivered
		message.DeliveredAt = &now
		event = convcontracts.RealtimeMessageDelivered
	case chentity.DeliveryRead:
		message.DeliveryStatus = conventity.DeliveryRead
		message.ReadAt = &now
		event = convcontracts.RealtimeMessageRead
	case chentity.DeliveryFailed:
		message.DeliveryStatus = conventity.DeliveryFailed
		message.DeliveryError = receipt.Error
		event = convcontracts.RealtimeMessageFailed
	default:
		return nil
	}
	if err := s.messages.Update(ctx, message); err != nil {
		return err
	}
	if conv != nil {
		s.publishStatus(ctx, conv, message, event, receipt.Error)
	}
	return nil
}

// recipient resolves the contact reference for an outbound send: the external id
// for the channel (falling back to the phone) plus name/phone for the envelope.
func (s *OutboundService) recipient(ctx context.Context, conv *conventity.Conversation, t chentity.Type) chcontracts.OutboundContact {
	contact, err := s.contacts.FindByID(ctx, conv.ContactID)
	if err != nil {
		return chcontracts.OutboundContact{}
	}
	externalID := contact.Phone
	for _, id := range contact.Identities {
		if id.Channel == string(t) {
			externalID = id.ExternalID
			break
		}
	}
	return chcontracts.OutboundContact{
		ID:         contact.ID,
		Name:       contact.Name,
		Phone:      contact.Phone,
		ExternalID: externalID,
	}
}

func (s *OutboundService) publishStatus(ctx context.Context, conv *conventity.Conversation, msg *conventity.Message, event, errMsg string) {
	payload := convcontracts.MessageStatusPayload{
		MessageID:      msg.ID,
		ConversationID: conv.ID,
		DeliveryStatus: string(msg.DeliveryStatus),
		Error:          errMsg,
	}
	_ = s.publisher.Publish(ctx, shared.TopicConversation(conv.TenantID, conv.ID), event, payload)
	if conv.AssignedTo != "" {
		_ = s.publisher.Publish(ctx, shared.TopicUser(conv.TenantID, conv.AssignedTo), event, payload)
	}
}

var _ convcontracts.OutboundDispatcher = (*OutboundService)(nil)
