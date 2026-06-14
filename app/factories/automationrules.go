package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	arservice "github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/service"
	infraautomationrules "github.com/romerito007/chat-smsnet-omnichannel/infra/automationrules"
	arrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/automationrules"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/contacts"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	webhookrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/webhooks"
	arctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/automationrules"
)

// AutomationRuleService builds the automation-rules service (CRUD + log reads). It
// validates referenced webhooks against the webhooks subscription repository.
func AutomationRuleService(c *container.Container) *arservice.RuleService {
	return arservice.NewRuleService(
		arrepo.NewRuleRepository(c.Mongo.DB),
		webhookrepo.NewSubscriptionRepository(c.Mongo.DB, c.Cipher),
		arrepo.NewLogRepository(c.Mongo.DB),
		clock,
	)
}

// AutomationRuleSink builds the event sink (Asynq enqueuer) that the conversation
// service calls to evaluate rules off the hot path.
func AutomationRuleSink(c *container.Container) *infraautomationrules.Enqueuer {
	return infraautomationrules.NewEnqueuer(c.AsynqClient)
}

// AutomationRuleEvaluator builds the async evaluator (the automationrule.evaluate
// worker handler). It reuses the webhooks dispatcher (EmitTo) for delivery and a
// Redis deduper for the anti-loop guard.
func AutomationRuleEvaluator(c *container.Container) *arservice.Evaluator {
	return arservice.NewEvaluator(
		arrepo.NewRuleRepository(c.Mongo.DB),
		arrepo.NewLogRepository(c.Mongo.DB),
		convrepo.NewConversationRepository(c.Mongo.DB),
		contactrepo.New(c.Mongo.DB),
		WebhookDispatcher(c),
		infraautomationrules.NewDeduper(c.Redis),
		clock,
	)
}

// AutomationRuleController builds the automation-rules CRUD + logs controller.
func AutomationRuleController(c *container.Container) *arctl.Controller {
	return arctl.NewController(AutomationRuleService(c))
}
