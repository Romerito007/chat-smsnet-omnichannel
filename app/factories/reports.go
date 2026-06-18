package factories

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	dcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/deals/contracts"
	dealservice "github.com/romerito007/chat-smsnet-omnichannel/domain/deals/service"
	rcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
	reportservice "github.com/romerito007/chat-smsnet-omnichannel/domain/reports/service"
	reportrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/reports"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/storage"
	reportctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/reports"
)

// ReportFileStore builds the export file store (local filesystem + signed URLs)
// pointing at the reports download endpoint.
func ReportFileStore(c *container.Container) rcontracts.FileStore {
	return storage.NewLocalFileStoreAt(
		c.Config.Reports.StorageDir,
		c.Config.Reports.SigningSecret,
		c.Config.Reports.DownloadBaseURL,
		"/v1/reports/downloads/",
	)
}

// ReportService builds the Mongo-aggregation report service with real file export.
// It is wired with the agent (IAM), sector and close-reason directories so every
// report — both the GET reads AND the file export — resolves raw ids to display
// names/labels (agent name+avatar, sector name, closed_by_reason label) in batch.
func ReportService(c *container.Container) *reportservice.Service {
	svc := reportservice.NewService(reportrepo.New(c.Mongo.DB), clock)
	svc.SetAuditor(AuditService(c))
	svc.SetFileStore(ReportFileStore(c), c.Config.Reports.DownloadTTL)
	svc.SetDirectories(UserService(c), SectorService(c), ConversationToolsCloseReasonService(c))
	// CRM sales-funnel reports are exportable through /reports/export too.
	svc.SetSalesReporter(salesReporter{metrics: SalesMetricsService(c)})
	return svc
}

// salesReporter adapts the deals SalesMetrics service to the reports export port.
type salesReporter struct{ metrics *dealservice.SalesMetrics }

func (a salesReporter) Funnel(ctx context.Context, from, to time.Time) (any, error) {
	return a.metrics.Funnel(ctx, dcontracts.SalesFilter{From: from, To: to})
}
func (a salesReporter) Agents(ctx context.Context, from, to time.Time) (any, error) {
	return a.metrics.Agents(ctx, dcontracts.SalesFilter{From: from, To: to})
}
func (a salesReporter) Cycle(ctx context.Context, from, to time.Time) (any, error) {
	return a.metrics.Cycle(ctx, dcontracts.SalesFilter{From: from, To: to}, 0)
}

// ReportController builds the reports controller. Name/label enrichment lives in the
// report service (shared by reads and export), so the controller just queries + writes.
func ReportController(c *container.Container) *reportctl.Controller {
	return reportctl.NewController(ReportService(c), ReportFileStore(c))
}
