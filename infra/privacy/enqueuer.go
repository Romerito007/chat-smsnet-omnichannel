// Package privacy holds the Asynq enqueuer for the privacy domain: it implements
// the contracts.ExportEnqueuer port (producers enqueue privacy.export). The
// privacy.retention job is enqueued by the scheduler, not here.
package privacy

import (
	"encoding/json"

	goasynq "github.com/hibiken/asynq"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/contracts"
	infraasynq "github.com/romerito007/chat-smsnet-omnichannel/infra/asynq"
)

// Enqueuer enqueues privacy.export tasks onto the reports queue (low priority,
// background work).
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
	t := goasynq.NewTask(infraasynq.TaskPrivacyExport, body)
	_, err = e.client.Enqueue(t, goasynq.Queue(infraasynq.QueueReports), goasynq.MaxRetry(5))
	return err
}

var _ contracts.ExportEnqueuer = (*Enqueuer)(nil)
