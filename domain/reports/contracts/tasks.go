package contracts

// ExportTask is the reports.export job payload: render a report for the given
// filter into a downloadable file (CSV/XLSX). The MVP enqueues and audits the
// request; the file generation itself is future work.
type ExportTask struct {
	TenantID string `json:"tenant_id"`
	ActorID  string `json:"actor_id"`
	Report   string `json:"report"` // overview|conversations|agents|...
	Format   string `json:"format"` // csv|xlsx
	From     int64  `json:"from"`   // unix millis
	To       int64  `json:"to"`
	SectorID string `json:"sector_id,omitempty"`
	Channel  string `json:"channel,omitempty"`
}

// ExportEnqueuer schedules the reports.export job. Implemented by the infra Asynq
// enqueuer.
type ExportEnqueuer interface {
	EnqueueExport(task ExportTask) error
}
