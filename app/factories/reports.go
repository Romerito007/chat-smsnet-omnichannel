package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	reportservice "github.com/romerito007/chat-smsnet-omnichannel/domain/reports/service"
	reportrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/reports"
	reportctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/reports"
)

// ReportService builds the Mongo-aggregation report service.
func ReportService(c *container.Container) *reportservice.Service {
	return reportservice.NewService(reportrepo.New(c.Mongo.DB), clock)
}

// ReportController builds the reports controller.
func ReportController(c *container.Container) *reportctl.Controller {
	return reportctl.NewController(ReportService(c))
}
