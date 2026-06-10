package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	channelservice "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/service"
	contactservice "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/service"
	infraautomation "github.com/romerito007/chat-smsnet-omnichannel/infra/automation"
	channelrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/channels"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/contacts"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	queuerepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/queues"
	channelctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/channels"
)

// ContactService builds the contact service.
func ContactService(c *container.Container) *contactservice.Service {
	return contactservice.New(contactrepo.New(c.Mongo.DB), clock)
}

// ChannelService builds the channel integration service.
func ChannelService(c *container.Container) *channelservice.ChannelService {
	return channelservice.NewChannelService(channelrepo.NewIntegrationRepository(c.Mongo.DB), clock)
}

// InboundService builds the inbound orchestration service.
func InboundService(c *container.Container) *channelservice.InboundService {
	return channelservice.NewInboundService(
		ContactService(c),
		convrepo.NewConversationRepository(c.Mongo.DB),
		convrepo.NewMessageRepository(c.Mongo.DB),
		convrepo.NewEventRepository(c.Mongo.DB),
		queuerepo.New(c.Mongo.DB),
		channelrepo.NewInboundRepository(c.Mongo.DB),
		infraautomation.NewDispatcher(c.AsynqClient),
		c.Locker,
		c.Events,
		clock,
	)
}

// ChannelController builds the integration management controller.
func ChannelController(c *container.Container) *channelctl.Controller {
	return channelctl.NewController(ChannelService(c))
}

// InboundController builds the public inbound controller.
func InboundController(c *container.Container) *channelctl.InboundController {
	return channelctl.NewInboundController(ChannelService(c), InboundService(c))
}
