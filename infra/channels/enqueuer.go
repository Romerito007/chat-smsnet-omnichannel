package channels

import (
	"encoding/json"
	"time"

	goasynq "github.com/hibiken/asynq"

	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	infraasynq "github.com/romerito007/chat-smsnet-omnichannel/infra/asynq"
)

// DeliveryEnqueuer enqueues channel.deliver / channel.retry tasks on the
// `channels` queue.
type DeliveryEnqueuer struct {
	client *infraasynq.Client
}

// NewDeliveryEnqueuer builds the enqueuer.
func NewDeliveryEnqueuer(client *infraasynq.Client) *DeliveryEnqueuer {
	return &DeliveryEnqueuer{client: client}
}

// EnqueueDeliver schedules an immediate delivery attempt.
func (e *DeliveryEnqueuer) EnqueueDeliver(payload chcontracts.DeliverTask) error {
	return e.enqueue(infraasynq.TaskChannelDeliver, payload, 0)
}

// EnqueueRetry schedules a retry after delaySeconds.
func (e *DeliveryEnqueuer) EnqueueRetry(payload chcontracts.DeliverTask, delaySeconds int) error {
	return e.enqueue(infraasynq.TaskChannelRetry, payload, time.Duration(delaySeconds)*time.Second)
}

func (e *DeliveryEnqueuer) enqueue(taskType string, payload chcontracts.DeliverTask, delay time.Duration) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	task := goasynq.NewTask(taskType, body)
	opts := []goasynq.Option{goasynq.Queue(infraasynq.QueueChannels), goasynq.MaxRetry(0)}
	if delay > 0 {
		opts = append(opts, goasynq.ProcessIn(delay))
	}
	_, err = e.client.Enqueue(task, opts...)
	return err
}

var _ chcontracts.DeliveryEnqueuer = (*DeliveryEnqueuer)(nil)
