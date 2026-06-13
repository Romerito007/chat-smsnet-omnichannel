package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	arservice "github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/service"
	arrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/automationrules"
	webhookrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/webhooks"
	arctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/automationrules"
)

// AutomationRuleService builds the automation-rules service. It validates
// referenced webhooks against the webhooks subscription repository.
func AutomationRuleService(c *container.Container) *arservice.RuleService {
	return arservice.NewRuleService(
		arrepo.NewRuleRepository(c.Mongo.DB),
		webhookrepo.NewSubscriptionRepository(c.Mongo.DB, c.Cipher),
		clock,
	)
}

// AutomationRuleController builds the automation-rules CRUD controller.
func AutomationRuleController(c *container.Container) *arctl.Controller {
	return arctl.NewController(AutomationRuleService(c))
}
