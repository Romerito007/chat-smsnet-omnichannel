package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	copilotentity "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	cservice "github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/service"
	infracopilot "github.com/romerito007/chat-smsnet-omnichannel/infra/copilot"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/copilot/provider"
	channelrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/channels"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/contacts"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	copilotrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/copilot"
	providerhubrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/providerhub"
	copilotctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/copilot"
)

// CopilotAssistantService builds the assistant service (many per tenant). It
// validates pinned ISP profiles against the providerhub profiles.
func CopilotAssistantService(c *container.Container) *cservice.AssistantService {
	return cservice.NewAssistantService(
		copilotrepo.NewAssistantRepository(c.Mongo.DB),
		providerhubrepo.NewProfileRepository(c.Mongo.DB, c.Cipher),
		channelrepo.NewConnectionRepository(c.Mongo.DB, c.Cipher),
		clock,
	)
}

// copilotISPToolBridge builds the SMSNET ISP tool bridge: it resolves a
// conversation's assistant → ISP profile to gate and inject the ISP config into
// MCP tool calls server-side.
func copilotISPToolBridge(c *container.Container) *cservice.ISPToolBridge {
	return cservice.NewISPToolBridge(
		copilotrepo.NewAssistantRepository(c.Mongo.DB),
		providerhubrepo.NewProfileRepository(c.Mongo.DB, c.Cipher),
	)
}

// CopilotAssistantController builds the assistant CRUD controller.
func CopilotAssistantController(c *container.Container) *copilotctl.AssistantController {
	return copilotctl.NewAssistantController(CopilotAssistantService(c))
}

// CopilotConfigService builds the per-tenant config service. The cipher encrypts
// the per-tenant provider API key at rest.
func CopilotConfigService(c *container.Container) *cservice.ConfigService {
	svc := cservice.NewConfigService(copilotrepo.NewConfigRepository(c.Mongo.DB, c.Cipher), clock)
	svc.SetAuditor(AuditService(c))
	return svc
}

// copilotRegistry builds the real provider registry. Adapters are stateless: the
// per-tenant API key/base URL travel on each request, so no env keys are wired
// and only real hosted providers are registered.
func copilotRegistry(_ *container.Container) *provider.Registry {
	return provider.NewRegistry()
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
	svc := cservice.NewService(
		CopilotConfigService(c),
		copilotrepo.NewLogRepository(c.Mongo.DB),
		convrepo.NewConversationRepository(c.Mongo.DB),
		builder,
		copilotRegistry(c),
		c.Events,
		clock,
	)
	// Wire the MCP tool broker so suggest_reply runs the agentic read-tool loop and
	// proposes write actions for approval.
	svc.SetToolBroker(MCPToolService(c))
	// Server-side logging of the real provider cause + env-default API keys used
	// when a tenant has selected a provider but set no key of its own.
	svc.SetLogger(c.Logger)
	svc.SetEnvKeys(map[copilotentity.Provider]string{
		copilotentity.ProviderOpenAI:    c.Config.Copilot.OpenAIKey,
		copilotentity.ProviderGemini:    c.Config.Copilot.GeminiKey,
		copilotentity.ProviderAnthropic: c.Config.Copilot.AnthropicKey,
	})
	return svc
}

// CopilotController builds the copilot controller.
func CopilotController(c *container.Container) *copilotctl.Controller {
	return copilotctl.NewController(CopilotConfigService(c), CopilotService(c))
}
