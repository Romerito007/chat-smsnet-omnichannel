package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	routingservice "github.com/romerito007/chat-smsnet-omnichannel/domain/routing/service"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/iam"
	presenceload "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/presence"
	queuerepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/queues"
	sectorrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/sectors"
	presencestore "github.com/romerito007/chat-smsnet-omnichannel/infra/presence"
	routingctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/routing"
)

// RoutingService builds the routing service.
func RoutingService(c *container.Container) *routingservice.Service {
	svc := routingservice.New(
		convrepo.NewConversationRepository(c.Mongo.DB),
		convrepo.NewEventRepository(c.Mongo.DB),
		presencestore.NewStore(c.Redis),
		presenceload.NewLoadCounter(c.Mongo.DB),
		iamrepo.NewUserRepository(c.Mongo.DB),
		sectorrepo.New(c.Mongo.DB),
		queuerepo.New(c.Mongo.DB),
		c.Locker,
		c.Events,
		clock,
	)
	svc.SetWebhookEmitter(WebhookDispatcher(c))
	svc.SetNotifier(NotificationEnqueuer(c))
	svc.SetAuditor(AuditService(c))
	return svc
}

// RoutingController builds the routing controller.
func RoutingController(c *container.Container) *routingctl.Controller {
	return routingctl.NewController(RoutingService(c))
}
