package contracts

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	routingcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/routing/contracts"
)

// Router is the subset of routing the automation service needs to apply
// assignment/transfer/enqueue decisions. The routing service satisfies it.
type Router interface {
	Assign(ctx context.Context, conversationID, agentID string) (*conventity.Conversation, error)
	Transfer(ctx context.Context, conversationID string, cmd routingcontracts.TransferCommand) (*conventity.Conversation, error)
	Enqueue(ctx context.Context, conversationID string, cmd routingcontracts.EnqueueCommand) (*conventity.Conversation, error)
}

// FlowInput is the payload sent to the external flow when starting a run.
type FlowInput struct {
	ConversationID string         `json:"conversation_id"`
	MessageID      string         `json:"message_id"`
	ContactID      string         `json:"contact_id"`
	Channel        string         `json:"channel"`
	Text           string         `json:"text"`
	CallbackURL    string         `json:"callback_url"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// FlowStartResult is the external flow's response to a start request. A non-nil
// Decision means the flow answered synchronously; otherwise the chat waits for a
// callback identified by ExternalRunID.
type FlowStartResult struct {
	ExternalRunID string
	Decision      *Decision
}

// FlowClient calls the external flow system. The implementation lives in
// infra/automation and signs requests with the integration secret.
type FlowClient interface {
	Start(ctx context.Context, integration *entity.AutomationIntegration, input FlowInput) (FlowStartResult, error)
}

// TimeoutTask is the Asynq payload for the automation timeout job.
type TimeoutTask struct {
	TenantID string `json:"tenant_id"`
	RunID    string `json:"run_id"`
}

// TimeoutScheduler schedules the (delayed) timeout-handling job for a run.
type TimeoutScheduler interface {
	ScheduleTimeout(task TimeoutTask, delayMs int) error
}
