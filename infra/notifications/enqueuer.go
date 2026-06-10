// Package notifications holds the Asynq enqueuer for the notifications domain:
// it implements the shared.Notifier port (producers enqueue notification.send)
// and the domain EmailEnqueuer port (notification.email). Both run on the
// `default` queue.
package notifications

import (
	"context"
	"encoding/json"

	goasynq "github.com/hibiken/asynq"

	ncontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	infraasynq "github.com/romerito007/chat-smsnet-omnichannel/infra/asynq"
)

// Enqueuer enqueues notification.send / notification.email tasks.
type Enqueuer struct {
	client *infraasynq.Client
}

// NewEnqueuer builds the enqueuer.
func NewEnqueuer(client *infraasynq.Client) *Enqueuer {
	return &Enqueuer{client: client}
}

// Notify implements shared.Notifier by enqueuing a notification.send task.
// Fire-and-forget: enqueue failures are swallowed.
func (e *Enqueuer) Notify(_ context.Context, in shared.NotifyInput) {
	if in.UserID == "" || in.TenantID == "" {
		return
	}
	_ = e.enqueue(infraasynq.TaskNotificationSend, ncontracts.SendTask{
		TenantID: in.TenantID,
		UserID:   in.UserID,
		Type:     in.Type,
		Title:    in.Title,
		Body:     in.Body,
		Link:     in.Link,
	})
}

// EnqueueEmail implements contracts.EmailEnqueuer.
func (e *Enqueuer) EnqueueEmail(task ncontracts.EmailTask) error {
	return e.enqueue(infraasynq.TaskNotificationEmail, task)
}

func (e *Enqueuer) enqueue(taskType string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	t := goasynq.NewTask(taskType, body)
	_, err = e.client.Enqueue(t, goasynq.Queue(infraasynq.QueueDefault), goasynq.MaxRetry(5))
	return err
}

var (
	_ shared.Notifier          = (*Enqueuer)(nil)
	_ ncontracts.EmailEnqueuer = (*Enqueuer)(nil)
)
