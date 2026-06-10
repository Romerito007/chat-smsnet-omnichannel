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
	presencectl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/presence"
	queuectl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/queues"
	sectorctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/sectors"
)

// SectorService builds the sector service.
func SectorService(c *container.Container) *sectorservice.Service {
	return sectorservice.New(sectorrepo.New(c.Mongo.DB), clock)
}

// QueueService builds the queue service (validates sectors).
func QueueService(c *container.Container) *queueservice.Service {
	return queueservice.New(queuerepo.New(c.Mongo.DB), sectorrepo.New(c.Mongo.DB), clock)
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

// PresenceController builds the presence controller.
func PresenceController(c *container.Container) *presencectl.Controller {
	return presencectl.NewController(PresenceService(c))
}
