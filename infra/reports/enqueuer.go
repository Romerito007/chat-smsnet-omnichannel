// Package reports holds the Asynq enqueuer for the reports domain: it implements
// the contracts.ExportEnqueuer port (the reports.export job).
package reports

import (
	"encoding/json"

	goasynq "github.com/hibiken/asynq"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
	infraasynq "github.com/romerito007/chat-smsnet-omnichannel/infra/asynq"
)

// Enqueuer enqueues reports.export tasks onto the reports queue.
type Enqueuer struct {
	client *infraasynq.Client
}

// NewEnqueuer builds the enqueuer.
func NewEnqueuer(client *infraasynq.Client) *Enqueuer {
	return &Enqueuer{client: client}
}

// EnqueueExport implements contracts.ExportEnqueuer.
func (e *Enqueuer) EnqueueExport(task contracts.ExportTask) error {
	body, err := json.Marshal(task)
	if err != nil {
		return err
	}
	t := goasynq.NewTask(infraasynq.TaskReportsExport, body)
	_, err = e.client.Enqueue(t, goasynq.Queue(infraasynq.QueueReports), goasynq.MaxRetry(3))
	return err
}

var _ contracts.ExportEnqueuer = (*Enqueuer)(nil)
