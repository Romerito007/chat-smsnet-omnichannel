package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	phservice "github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/service"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/contacts"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	providerhubrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/providerhub"
	infraproviderhub "github.com/romerito007/chat-smsnet-omnichannel/infra/providerhub"
	providerhubctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/providerhub"
)

// providerHubGateway builds the HTTP gateway with the structured logger wired, so a
// failing SMSNET call records its real cause (status/url/body) instead of only the
// opaque "serviço indisponível".
func providerHubGateway(c *container.Container) *infraproviderhub.Gateway {
	g := infraproviderhub.NewGateway()
	g.SetLogger(c.Logger)
	return g
}

// ProviderHubProfileService builds the ISP-profile service (CRUD + gateway status).
func ProviderHubProfileService(c *container.Container) *phservice.ProfileService {
	svc := phservice.NewProfileService(
		providerhubrepo.NewProfileRepository(c.Mongo.DB, c.Cipher),
		providerHubGateway(c),
		clock,
	)
	svc.SetEnvDefault(c.Config.ProviderHub.GatewayAPIHost, c.Config.ProviderHub.GatewayAPIKey)
	// Block deleting a profile that a CopilotAssistant pins (clear "in use" error).
	svc.SetUsageChecker(CopilotAssistantService(c))
	return svc
}

// ProviderHubQueryService builds the on-demand query service.
func ProviderHubQueryService(c *container.Container) *phservice.QueryService {
	svc := phservice.NewQueryService(
		providerhubrepo.NewProfileRepository(c.Mongo.DB, c.Cipher),
		providerhubrepo.NewQueryLogRepository(c.Mongo.DB),
		convrepo.NewConversationRepository(c.Mongo.DB),
		contactrepo.New(c.Mongo.DB),
		providerHubGateway(c),
		infraproviderhub.NewRateLimiter(c.Redis, c.Config.ProviderHub.RatePerMinute),
		clock,
	)
	svc.SetEnvDefault(c.Config.ProviderHub.GatewayAPIHost, c.Config.ProviderHub.GatewayAPIKey)
	svc.SetAuditor(AuditService(c))
	return svc
}

// ProviderHubController builds the providerhub controller.
func ProviderHubController(c *container.Container) *providerhubctl.Controller {
	return providerhubctl.NewController(ProviderHubProfileService(c), ProviderHubQueryService(c))
}
