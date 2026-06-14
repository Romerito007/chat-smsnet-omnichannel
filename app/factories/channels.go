package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	channelservice "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/service"
	contactservice "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/service"
	infrachannels "github.com/romerito007/chat-smsnet-omnichannel/infra/channels"
	channelrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/channels"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/contacts"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	channelctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/channels"
)

// channelRegistry is the shared adapter registry (stateless).
func channelRegistry() chcontracts.AdapterRegistry { return infrachannels.NewRegistry() }

// ContactService builds the contact service.
func ContactService(c *container.Container) *contactservice.Service {
	svc := contactservice.New(contactrepo.New(c.Mongo.DB), clock)
	svc.SetAuditor(AuditService(c))
	// Normalize contact tags to canonical ids (catalog names -> ids, free labels kept).
	svc.SetTagResolver(ConversationToolsTagService(c))
	// Validate a contact avatar attachment (exists, same tenant, image, ready).
	svc.SetAvatarValidator(AttachmentService(c))
	// Resolve contact avatars to short-lived signed URLs in the response payloads.
	svc.SetAvatarURLResolver(AttachmentService(c))
	// Validate custom_attributes against applies_to=contact definitions.
	svc.SetCustomAttributeValidator(CustomAttributeService(c))
	return svc
}

// ConnectionService builds the channel connection service, wired to the HTTP
// health checker used by the channels.health_check job.
func ConnectionService(c *container.Container) *channelservice.ConnectionService {
	svc := channelservice.NewConnectionService(
		channelrepo.NewConnectionRepository(c.Mongo.DB, c.Cipher),
		channelRegistry(),
		clock,
	)
	svc.SetHealthChecker(infrachannels.NewHealthChecker())
	svc.SetAuditor(AuditService(c))
	return svc
}

// InboundService builds the inbound orchestration service.
func InboundService(c *container.Container) *channelservice.InboundService {
	svc := channelservice.NewInboundService(
		ContactService(c),
		convrepo.NewConversationRepository(c.Mongo.DB),
		convrepo.NewMessageRepository(c.Mongo.DB),
		convrepo.NewEventRepository(c.Mongo.DB),
		convrepo.NewProtocolCounterRepository(c.Mongo.DB),
		channelrepo.NewInboundRepository(c.Mongo.DB),
		c.Locker,
		c.Events,
		clock,
	)
	// Raw (multipart) inbound attachments are persisted via the attachments service.
	svc.SetAttachmentStore(AttachmentService(c))
	// Inbound lifecycle (conversation created/reopened) feeds the automation-rules
	// engine.
	svc.SetRuleSink(AutomationRuleSink(c))
	return svc
}

// OutboundService builds the outbound delivery service.
func OutboundService(c *container.Container) *channelservice.OutboundService {
	svc := channelservice.NewOutboundService(
		channelrepo.NewConnectionRepository(c.Mongo.DB, c.Cipher),
		channelrepo.NewOutboundDeliveryRepository(c.Mongo.DB),
		convrepo.NewConversationRepository(c.Mongo.DB),
		convrepo.NewMessageRepository(c.Mongo.DB),
		contactrepo.New(c.Mongo.DB),
		channelRegistry(),
		infrachannels.NewDeliveryEnqueuer(c.AsynqClient),
		c.Events,
		clock,
	)
	svc.SetNotifier(NotificationEnqueuer(c))
	// Outbound integration media is delivered as signed, public (JWT-less) URLs.
	svc.SetMediaURLBuilder(AttachmentService(c))
	return svc
}

// ConnectionController builds the connection management controller.
func ConnectionController(c *container.Container) *channelctl.ConnectionController {
	return channelctl.NewConnectionController(ConnectionService(c))
}

// InboundController builds the public inbound controller (messages + receipts).
func InboundController(c *container.Container) *channelctl.InboundController {
	return channelctl.NewInboundController(ConnectionService(c), InboundService(c), OutboundService(c))
}
