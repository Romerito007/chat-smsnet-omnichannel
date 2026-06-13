// Package automationrules holds the infra for the automation-rules engine: the
// event sink (Asynq enqueuer) and the Redis anti-loop deduper.
package automationrules

import (
	"context"
	"encoding/json"

	goasynq "github.com/hibiken/asynq"

	arcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	infraasynq "github.com/romerito007/chat-smsnet-omnichannel/infra/asynq"
)

// Enqueuer implements shared.RuleEventSink: it serializes a conversation event
// into an automationrule.evaluate task on the default queue, so rule evaluation
// runs off the emitting service's hot path. Emission is best-effort.
type Enqueuer struct {
	client *infraasynq.Client
}

// NewEnqueuer builds the enqueuer.
func NewEnqueuer(client *infraasynq.Client) *Enqueuer {
	return &Enqueuer{client: client}
}

// EmitRuleEvent enqueues the evaluation task. The payload travels as the webhook
// data; the worker hydrates the conversation/contact for condition matching.
func (e *Enqueuer) EmitRuleEvent(_ context.Context, tenantID, event, conversationID string, payload any) {
	if tenantID == "" || conversationID == "" {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	body, err := json.Marshal(arcontracts.EvaluateTask{
		TenantID:       tenantID,
		Event:          event,
		ConversationID: conversationID,
		Data:           data,
	})
	if err != nil {
		return
	}
	t := goasynq.NewTask(infraasynq.TaskRuleEvaluate, body)
	_, _ = e.client.Enqueue(t, goasynq.Queue(infraasynq.QueueDefault), goasynq.MaxRetry(3))
}

var _ shared.RuleEventSink = (*Enqueuer)(nil)
