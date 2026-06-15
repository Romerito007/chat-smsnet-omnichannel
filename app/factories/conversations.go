package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	convservice "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/service"
	channelrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/channels"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	sectorrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/sectors"
	convctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/conversations"
)

// conversationServiceBase builds the conversations service with every wiring
// EXCEPT the CSAT trigger. CSAT's channel sender reuses this base (it only needs
// SendSystemMessage), which breaks the conversations<->csat construction cycle.
func conversationServiceBase(c *container.Container) *convservice.Service {
	svc := convservice.New(
		convrepo.NewConversationRepository(c.Mongo.DB),
		convrepo.NewMessageRepository(c.Mongo.DB),
		convrepo.NewEventRepository(c.Mongo.DB),
		sectorrepo.New(c.Mongo.DB),
		channelrepo.NewConnectionRepository(c.Mongo.DB, c.Cipher),
		c.Events,
		clock,
	)
	svc.SetWebhookEmitter(WebhookDispatcher(c))
	// Enrich outbound webhook payloads with the recipient contact (+ identities) and
	// the agent (id+name) — resolved lazily, only when a subscription matches.
	svc.SetWebhookEnricher(WebhookEnricher(c))
	// Outbound webhook payloads carry signed, public channel-media URLs so the
	// integrator can fetch a delivered message's attachments without a JWT.
	svc.SetIntegrationMediaResolver(AttachmentService(c))
	// Evaluate automation rules off the hot path (async via Asynq).
	svc.SetRuleEventSink(AutomationRuleSink(c))
	svc.SetTagCatalog(ConversationToolsTagService(c))
	svc.SetCloseReasonPolicy(ConversationToolsCloseReasonService(c))
	svc.SetSLAHook(SLAService(c))
	svc.SetNotifier(NotificationEnqueuer(c))
	svc.SetAuditor(AuditService(c))
	svc.SetQueueStatsNotifier(QueueService(c))
	// Hydrate message attachments (url/content_type/filename/size) at read and
	// validate attachment ids on send.
	svc.SetAttachmentResolver(AttachmentService(c))
	// Resolve the contact + assignee display cards per inbox row (batch).
	svc.SetContactDirectory(ContactService(c))
	svc.SetAgentDirectory(UserService(c))
	// Validate custom_attributes against applies_to=conversation definitions.
	svc.SetCustomAttributeValidator(CustomAttributeService(c))
	return svc
}

// ConversationService builds the full conversations service, including the CSAT
// close trigger so closing an eligible conversation starts a satisfaction survey.
func ConversationService(c *container.Container) *convservice.Service {
	svc := conversationServiceBase(c)
	svc.SetCSATTrigger(CSATService(c))
	return svc
}

// ConversationController builds the conversations controller.
func ConversationController(c *container.Container) *convctl.Controller {
	return convctl.NewController(ConversationService(c))
}
