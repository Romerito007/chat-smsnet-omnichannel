package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	slaservice "github.com/romerito007/chat-smsnet-omnichannel/domain/sla/service"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	slarepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/sla"
	slactl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/sla"
)

// SLAPolicyService builds the SLA policy CRUD service.
func SLAPolicyService(c *container.Container) *slaservice.PolicyService {
	return slaservice.NewPolicyService(slarepo.NewPolicyRepository(c.Mongo.DB), clock)
}

// SLAService builds the SLA tracking/check service (also the conversations
// SLAHook). Due dates are computed in business time via the businesshours
// service; breaches publish realtime + the sla.breached webhook.
func SLAService(c *container.Container) *slaservice.Service {
	svc := slaservice.NewService(
		slarepo.NewPolicyRepository(c.Mongo.DB),
		slarepo.NewTrackingRepository(c.Mongo.DB),
		convrepo.NewConversationRepository(c.Mongo.DB),
		BusinessHoursService(c),
		c.Events,
		WebhookDispatcher(c),
		clock,
	)
	svc.SetNotifier(NotificationEnqueuer(c))
	return svc
}

// SLAController builds the SLA controller.
func SLAController(c *container.Container) *slactl.Controller {
	return slactl.NewController(SLAPolicyService(c), SLAService(c))
}
