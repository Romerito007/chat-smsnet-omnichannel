package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	bhservice "github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/service"
	bhrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/businesshours"
	channelrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/channels"
	bhctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/businesshours"
)

// HolidayService builds the holiday CRUD service.
func HolidayService(c *container.Container) *bhservice.HolidayService {
	return bhservice.NewHolidayService(bhrepo.NewHolidayRepository(c.Mongo.DB), clock)
}

// BusinessHoursService builds the timezone/holiday-aware business-hours service
// (also the shared BusinessHoursChecker/BusinessClock consulted by SLA/automation).
// Business hours live on the ChannelConnection, so it reads from the channels repo.
func BusinessHoursService(c *container.Container) *bhservice.BusinessHoursService {
	return bhservice.NewBusinessHoursService(
		channelrepo.NewConnectionRepository(c.Mongo.DB, c.Cipher),
		bhrepo.NewHolidayRepository(c.Mongo.DB),
		clock,
	)
}

// BusinessHoursController builds the businesshours controller.
func BusinessHoursController(c *container.Container) *bhctl.Controller {
	return bhctl.NewController(HolidayService(c), BusinessHoursService(c))
}
