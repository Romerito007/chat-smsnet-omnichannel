package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	wservice "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/service"
	webhookrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/webhooks"
	infrawebhooks "github.com/romerito007/chat-smsnet-omnichannel/infra/webhooks"
	webhookctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/webhooks"
)

// WebhookSubscriptionService builds the subscription CRUD + test service.
func WebhookSubscriptionService(c *container.Container) *wservice.SubscriptionService {
	svc := wservice.NewSubscriptionService(
		webhookrepo.NewSubscriptionRepository(c.Mongo.DB, c.Cipher),
		webhookrepo.NewDeliveryRepository(c.Mongo.DB),
		infrawebhooks.NewSender(),
		clock,
	)
	svc.SetAuditor(AuditService(c))
	// Block deleting a webhook an automation rule references (clear 409).
	svc.SetUsageChecker(AutomationRuleService(c))
	return svc
}

// WebhookDispatcher builds the internal-event dispatcher (a shared.WebhookEmitter).
func WebhookDispatcher(c *container.Container) *wservice.Dispatcher {
	return wservice.NewDispatcher(
		webhookrepo.NewSubscriptionRepository(c.Mongo.DB, c.Cipher),
		webhookrepo.NewDeliveryRepository(c.Mongo.DB),
		infrawebhooks.NewEnqueuer(c.AsynqClient),
		clock,
	)
}

// WebhookDeliveryService builds the delivery worker service.
func WebhookDeliveryService(c *container.Container) *wservice.DeliveryService {
	return wservice.NewDeliveryService(
		webhookrepo.NewSubscriptionRepository(c.Mongo.DB, c.Cipher),
		webhookrepo.NewDeliveryRepository(c.Mongo.DB),
		infrawebhooks.NewSender(),
		infrawebhooks.NewEnqueuer(c.AsynqClient),
		infrawebhooks.NewRateLimiter(c.Redis),
		clock,
	)
}

// WebhookController builds the webhooks controller.
func WebhookController(c *container.Container) *webhookctl.Controller {
	return webhookctl.NewController(WebhookSubscriptionService(c))
}
