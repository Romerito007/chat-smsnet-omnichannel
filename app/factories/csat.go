package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	cservice "github.com/romerito007/chat-smsnet-omnichannel/domain/csat/service"
	infracsat "github.com/romerito007/chat-smsnet-omnichannel/infra/csat"
	csatrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/csat"
	csatctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/csat"
)

// CSATSurveyService builds the survey CRUD service.
func CSATSurveyService(c *container.Container) *cservice.SurveyService {
	return cservice.NewSurveyService(csatrepo.NewSurveyRepository(c.Mongo.DB), clock)
}

// CSATService builds the CSAT flow service (the conversations CSATTrigger plus
// the csat.send / csat.expire handlers and the public answer). The survey is
// delivered through the conversations -> channels outbound path.
func CSATService(c *container.Container) *cservice.Service {
	return cservice.NewService(
		csatrepo.NewSurveyRepository(c.Mongo.DB),
		csatrepo.NewResponseRepository(c.Mongo.DB),
		infracsat.NewChannelSender(conversationServiceBase(c)),
		infracsat.NewEnqueuer(c.AsynqClient),
		clock,
		c.Config.CSAT.ExpireAfterSeconds,
		c.Config.CSAT.PublicBaseURL,
	)
}

// CSATController builds the CSAT controller.
func CSATController(c *container.Container) *csatctl.Controller {
	return csatctl.NewController(CSATSurveyService(c), CSATService(c))
}
