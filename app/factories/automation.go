package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	autorepo "github.com/romerito007/chat-smsnet-omnichannel/domain/automation/repository"
	automationservice "github.com/romerito007/chat-smsnet-omnichannel/domain/automation/service"
	infraautomation "github.com/romerito007/chat-smsnet-omnichannel/infra/automation"
	automationrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/automation"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	automationctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/automation"
)

// automationRunRepo builds the run repository.
func automationRunRepo(c *container.Container) autorepo.RunRepository {
	return automationrepo.NewRunRepository(c.Mongo.DB)
}

// AutomationIntegrationService builds the integration CRUD service.
func AutomationIntegrationService(c *container.Container) *automationservice.IntegrationService {
	return automationservice.NewIntegrationService(
		automationrepo.NewIntegrationRepository(c.Mongo.DB, c.Cipher),
		clock,
	)
}

// AutomationService builds the automation run service.
func AutomationService(c *container.Container) *automationservice.Service {
	return automationservice.New(
		automationrepo.NewIntegrationRepository(c.Mongo.DB, c.Cipher),
		automationRunRepo(c),
		convrepo.NewConversationRepository(c.Mongo.DB),
		convrepo.NewMessageRepository(c.Mongo.DB),
		convrepo.NewEventRepository(c.Mongo.DB),
		RoutingService(c),
		OutboundService(c),
		infraautomation.NewFlowClient(),
		infraautomation.NewTimeoutScheduler(c.AsynqClient),
		c.Events,
		clock,
		c.Config.Automation.CallbackBaseURL,
	)
}

// AutomationController builds the automation controller.
func AutomationController(c *container.Container) *automationctl.Controller {
	return automationctl.NewController(AutomationIntegrationService(c), AutomationService(c), automationRunRepo(c))
}
