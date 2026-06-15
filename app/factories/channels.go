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
	// Contact create/update fan out to webhooks (contact_created / contact_updated).
	svc.SetWebhookEmitter(WebhookDispatcher(c))
	return svc
}

// ConnectionService builds the channel connection service, wired to the HTTP
// health checker used by the channels.health_check job and to the webhook manager
// that keeps the channel's managed (outbound-URL) webhook subscription in sync.
func ConnectionService(c *container.Container) *channelservice.ConnectionService {
	svc := channelservice.NewConnectionService(
		channelrepo.NewConnectionRepository(c.Mongo.DB, c.Cipher),
		channelRegistry(),
		clock,
	)
	svc.SetHealthChecker(infrachannels.NewHealthChecker())
	svc.SetAuditor(AuditService(c))
	// A channel with an outbound URL produces a managed webhook (full pipeline)
	// instead of a separate outbound rail.
	svc.SetWebhookManager(WebhookSubscriptionService(c))
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
	// Inbound messages + conversation lifecycle fan out to webhooks (Chatwoot
	// model), with signed channel-media attachment URLs in the payload.
	svc.SetWebhookEmitter(WebhookDispatcher(c))
	// Enrich inbound webhook payloads with the recipient contact (+ identities),
	// resolved lazily — only when a subscription matches the event.
	svc.SetWebhookEnricher(WebhookEnricher(c))
	svc.SetIntegrationMediaResolver(AttachmentService(c))
	return svc
}

// ConnectionController builds the connection management controller.
func ConnectionController(c *container.Container) *channelctl.ConnectionController {
	return channelctl.NewConnectionController(ConnectionService(c))
}

// InboundController builds the public inbound controller (messages + receipts).
// Delivery receipts (optional, by chat message_id) apply message status through
// the conversations service.
func InboundController(c *container.Container) *channelctl.InboundController {
	return channelctl.NewInboundController(ConnectionService(c), InboundService(c), ConversationService(c))
}
