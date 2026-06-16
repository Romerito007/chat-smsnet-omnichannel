package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	presenceservice "github.com/romerito007/chat-smsnet-omnichannel/domain/presence/service"
	queueservice "github.com/romerito007/chat-smsnet-omnichannel/domain/queues/service"
	sectorservice "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/service"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/iam"
	presenceload "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/presence"
	queuerepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/queues"
	sectorrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/sectors"
	presencestore "github.com/romerito007/chat-smsnet-omnichannel/infra/presence"
	agentsctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/agents"
	presencectl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/presence"
	queuectl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/queues"
	sectorctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/sectors"
)

// SectorService builds the sector service.
func SectorService(c *container.Container) *sectorservice.Service {
	return sectorservice.New(sectorrepo.New(c.Mongo.DB), clock)
}

// QueueService builds the queue service (validates sectors) wired to publish
// queue.stats from the realtime publisher + conversation composition counter.
func QueueService(c *container.Container) *queueservice.Service {
	svc := queueservice.New(queuerepo.New(c.Mongo.DB), sectorrepo.New(c.Mongo.DB), clock)
	svc.SetStats(c.Events, queuerepo.NewCompositionCounter(c.Mongo.DB))
	return svc
}

// PresenceService builds the presence service (Redis store + Mongo load counter
// + realtime publisher).
func PresenceService(c *container.Container) *presenceservice.Service {
	return presenceservice.New(
		presencestore.NewStore(c.Redis),
		presenceload.NewLoadCounter(c.Mongo.DB),
		iamrepo.NewUserRepository(c.Mongo.DB),
		c.Events,
		clock,
	)
}

// SectorController builds the sector controller.
func SectorController(c *container.Container) *sectorctl.Controller {
	return sectorctl.NewController(SectorService(c))
}

// QueueController builds the queue controller.
func QueueController(c *container.Container) *queuectl.Controller {
	return queuectl.NewController(QueueService(c))
}

// PresenceController builds the presence controller, wired with the IAM user
// directory so each presence row resolves the agent name + avatar instead of
// returning a raw user id.
func PresenceController(c *container.Container) *presencectl.Controller {
	return presencectl.NewController(PresenceService(c)).
		SetAgentDirectory(UserService(c))
}

// PresenceExpiryWatcher builds the Redis keyspace watcher that turns an expired
// presence TTL (an agent whose WS heartbeat stopped) into a live offline event.
func PresenceExpiryWatcher(c *container.Container) *presencestore.ExpiryWatcher {
	return presencestore.NewExpiryWatcher(c.Redis, c.Config.Redis.DB, PresenceService(c), c.Logger)
}

// AgentsController builds the assignable-agents directory controller (users +
// presence), read by the assignment selector under conversation.assign.
func AgentsController(c *container.Container) *agentsctl.Controller {
	return agentsctl.NewController(UserService(c), PresenceService(c))
}
