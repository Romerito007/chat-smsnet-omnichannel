package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	mservice "github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/service"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/contacts"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	monitoringrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/monitoring"
	inframonitoring "github.com/romerito007/chat-smsnet-omnichannel/infra/monitoring"
	monitoringctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/monitoring"
)

// MonitoringConfigService builds the config service.
func MonitoringConfigService(c *container.Container) *mservice.ConfigService {
	return mservice.NewConfigService(
		monitoringrepo.NewConfigRepository(c.Mongo.DB, c.Cipher),
		monitoringrepo.NewQueryLogRepository(c.Mongo.DB),
		inframonitoring.NewGateway(),
		clock,
	)
}

// MonitoringQueryService builds the on-demand query service.
func MonitoringQueryService(c *container.Container) *mservice.QueryService {
	return mservice.NewQueryService(
		monitoringrepo.NewConfigRepository(c.Mongo.DB, c.Cipher),
		monitoringrepo.NewQueryLogRepository(c.Mongo.DB),
		convrepo.NewConversationRepository(c.Mongo.DB),
		contactrepo.New(c.Mongo.DB),
		inframonitoring.NewGateway(),
		inframonitoring.NewRateLimiter(c.Redis, c.Config.Monitoring.RatePerMinute),
		clock,
	)
}

// MonitoringController builds the monitoring controller.
func MonitoringController(c *container.Container) *monitoringctl.Controller {
	return monitoringctl.NewController(MonitoringConfigService(c), MonitoringQueryService(c))
}
