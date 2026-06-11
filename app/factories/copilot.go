package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	cservice "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/service"
	infracopilot "github.com/romerito007/chat-smsnet-omnichannel/infra/copilot"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/copilot/provider"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/contacts"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	copilotrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/copilot"
	copilotctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/copilot"
)

// CopilotConfigService builds the per-tenant config service.
func CopilotConfigService(c *container.Container) *cservice.ConfigService {
	svc := cservice.NewConfigService(copilotrepo.NewConfigRepository(c.Mongo.DB), clock)
	svc.SetAuditor(AuditService(c))
	return svc
}

// copilotRegistry builds the provider registry from the configured API keys.
func copilotRegistry(c *container.Container) *provider.Registry {
	return provider.NewRegistry(provider.Keys{
		OpenAI:    c.Config.Copilot.OpenAIKey,
		Gemini:    c.Config.Copilot.GeminiKey,
		Anthropic: c.Config.Copilot.AnthropicKey,
	})
}

// CopilotService builds the copilot inference service. The context builder is
// wired with the customer data source; financial/monitoring enrichment is left
// unwired in the MVP (the builder still enforces every allow_*_data policy).
func CopilotService(c *container.Container) *cservice.Service {
	builder := cservice.NewContextBuilder(
		convrepo.NewMessageRepository(c.Mongo.DB),
		infracopilot.NewCustomerSource(contactrepo.New(c.Mongo.DB)),
		nil, // financial source: unwired in MVP
		nil, // monitoring source: unwired in MVP
	)
	return cservice.NewService(
		CopilotConfigService(c),
		copilotrepo.NewLogRepository(c.Mongo.DB),
		convrepo.NewConversationRepository(c.Mongo.DB),
		builder,
		copilotRegistry(c),
		c.Events,
		clock,
	)
}

// CopilotController builds the copilot controller.
func CopilotController(c *container.Container) *copilotctl.Controller {
	return copilotctl.NewController(CopilotConfigService(c), CopilotService(c))
}
