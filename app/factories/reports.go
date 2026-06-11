package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	reportservice "github.com/romerito007/chat-smsnet-omnichannel/domain/reports/service"
	reportrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/reports"
	infrareports "github.com/romerito007/chat-smsnet-omnichannel/infra/reports"
	reportctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/reports"
)

// ReportService builds the Mongo-aggregation report service.
func ReportService(c *container.Container) *reportservice.Service {
	svc := reportservice.NewService(reportrepo.New(c.Mongo.DB), clock)
	svc.SetAuditor(AuditService(c))
	svc.SetExportEnqueuer(infrareports.NewEnqueuer(c.AsynqClient))
	return svc
}

// ReportController builds the reports controller.
func ReportController(c *container.Container) *reportctl.Controller {
	return reportctl.NewController(ReportService(c))
}
