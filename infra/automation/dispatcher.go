// Package automation contains the Asynq-backed dispatcher that defers the
// (slow, non-critical) external flow invocation out of the inbound fast path.
// The actual flow lives in an external system; here we only enqueue and, in the
// worker, would call out to it.
package automation

import (
	"encoding/json"

	goasynq "github.com/hibiken/asynq"

	chcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	infraasynq "github.com/romerito007/chat-smsnet-omnichannel/infra/asynq"
)

// Dispatcher enqueues automation.invoke tasks.
type Dispatcher struct {
	client *infraasynq.Client
}

// NewDispatcher builds the dispatcher.
func NewDispatcher(client *infraasynq.Client) *Dispatcher {
	return &Dispatcher{client: client}
}

// Dispatch enqueues the automation invocation on the default (non-critical) queue.
func (d *Dispatcher) Dispatch(payload chcontracts.AutomationInvoke) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	task := goasynq.NewTask(infraasynq.TaskAutomationInvoke, body)
	_, err = d.client.Enqueue(task, goasynq.Queue(infraasynq.QueueDefault), goasynq.MaxRetry(5))
	return err
}

var _ chcontracts.AutomationDispatcher = (*Dispatcher)(nil)
