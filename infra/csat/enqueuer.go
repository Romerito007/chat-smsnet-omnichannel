// Package csat holds the CSAT Asynq enqueuer (csat.send / csat.expire) and the
// channel-sender adapter that delivers the survey through the conversations /
// channels outbound path (Prompt 9).
package csat

import (
	"encoding/json"
	"time"

	goasynq "github.com/hibiken/asynq"

	ccontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/csat/contracts"
	infraasynq "github.com/romerito007/chat-smsnet-omnichannel/infra/asynq"
)

// Enqueuer schedules csat.send / csat.expire tasks on the default queue.
type Enqueuer struct {
	client *infraasynq.Client
}

// NewEnqueuer builds the enqueuer.
func NewEnqueuer(client *infraasynq.Client) *Enqueuer {
	return &Enqueuer{client: client}
}

// EnqueueSend schedules the survey send after delaySeconds.
func (e *Enqueuer) EnqueueSend(task ccontracts.SendTask, delaySeconds int) error {
	return e.enqueue(infraasynq.TaskCSATSend, task, delaySeconds)
}

// EnqueueExpire schedules the expiry check after delaySeconds.
func (e *Enqueuer) EnqueueExpire(task ccontracts.ExpireTask, delaySeconds int) error {
	return e.enqueue(infraasynq.TaskCSATExpire, task, delaySeconds)
}

func (e *Enqueuer) enqueue(taskType string, payload any, delaySeconds int) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	t := goasynq.NewTask(taskType, body)
	opts := []goasynq.Option{goasynq.Queue(infraasynq.QueueDefault), goasynq.MaxRetry(5)}
	if delaySeconds > 0 {
		opts = append(opts, goasynq.ProcessIn(time.Duration(delaySeconds)*time.Second))
	}
	_, err = e.client.Enqueue(t, opts...)
	return err
}

var _ ccontracts.Enqueuer = (*Enqueuer)(nil)
