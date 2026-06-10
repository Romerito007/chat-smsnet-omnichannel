package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	bhservice "github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/service"
	bhrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/businesshours"
	sectorrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/sectors"
	bhctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/businesshours"
)

// HolidayService builds the holiday CRUD service.
func HolidayService(c *container.Container) *bhservice.HolidayService {
	return bhservice.NewHolidayService(bhrepo.NewHolidayRepository(c.Mongo.DB), clock)
}

// BusinessHoursService builds the timezone/holiday-aware business-hours service
// (also the shared BusinessHoursChecker consulted by routing/automation).
func BusinessHoursService(c *container.Container) *bhservice.BusinessHoursService {
	return bhservice.NewBusinessHoursService(
		sectorrepo.New(c.Mongo.DB),
		bhrepo.NewHolidayRepository(c.Mongo.DB),
		clock,
	)
}

// BusinessHoursController builds the businesshours controller.
func BusinessHoursController(c *container.Container) *bhctl.Controller {
	return bhctl.NewController(HolidayService(c), BusinessHoursService(c))
}
