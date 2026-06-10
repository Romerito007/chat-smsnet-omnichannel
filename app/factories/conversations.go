package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	convservice "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/service"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	sectorrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/sectors"
	convctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/conversations"
)

// ConversationService builds the conversations service, wired to the channels
// outbound dispatcher so agent messages are delivered to the customer's channel.
func ConversationService(c *container.Container) *convservice.Service {
	svc := convservice.New(
		convrepo.NewConversationRepository(c.Mongo.DB),
		convrepo.NewMessageRepository(c.Mongo.DB),
		convrepo.NewEventRepository(c.Mongo.DB),
		sectorrepo.New(c.Mongo.DB),
		c.Events,
		clock,
	)
	svc.SetOutboundDispatcher(OutboundService(c))
	svc.SetWebhookEmitter(WebhookDispatcher(c))
	svc.SetTagCatalog(ConversationToolsTagService(c))
	svc.SetCloseReasonPolicy(ConversationToolsCloseReasonService(c))
	svc.SetSLAHook(SLAService(c))
	svc.SetNotifier(NotificationEnqueuer(c))
	return svc
}

// ConversationController builds the conversations controller.
func ConversationController(c *container.Container) *convctl.Controller {
	return convctl.NewController(ConversationService(c))
}
