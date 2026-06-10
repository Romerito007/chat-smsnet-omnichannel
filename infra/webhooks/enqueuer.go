package webhooks

import (
	"encoding/json"
	"time"

	goasynq "github.com/hibiken/asynq"

	whcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/contracts"
	infraasynq "github.com/romerito007/chat-smsnet-omnichannel/infra/asynq"
)

// Enqueuer enqueues webhook.deliver / webhook.retry tasks on the `webhooks`
// queue. Asynq retries are disabled (MaxRetry 0): the domain delivery service
// owns retry/backoff and dead-lettering explicitly.
type Enqueuer struct {
	client *infraasynq.Client
}

// NewEnqueuer builds the enqueuer.
func NewEnqueuer(client *infraasynq.Client) *Enqueuer {
	return &Enqueuer{client: client}
}

// EnqueueDeliver schedules an immediate delivery attempt.
func (e *Enqueuer) EnqueueDeliver(task whcontracts.DeliverTask) error {
	return e.enqueue(infraasynq.TaskWebhookDeliver, task, 0)
}

// EnqueueRetry schedules a retry after delaySeconds.
func (e *Enqueuer) EnqueueRetry(task whcontracts.DeliverTask, delaySeconds int) error {
	return e.enqueue(infraasynq.TaskWebhookRetry, task, time.Duration(delaySeconds)*time.Second)
}

func (e *Enqueuer) enqueue(taskType string, task whcontracts.DeliverTask, delay time.Duration) error {
	body, err := json.Marshal(task)
	if err != nil {
		return err
	}
	t := goasynq.NewTask(taskType, body)
	opts := []goasynq.Option{goasynq.Queue(infraasynq.QueueWebhooks), goasynq.MaxRetry(0)}
	if delay > 0 {
		opts = append(opts, goasynq.ProcessIn(delay))
	}
	_, err = e.client.Enqueue(t, opts...)
	return err
}

var _ whcontracts.Enqueuer = (*Enqueuer)(nil)
