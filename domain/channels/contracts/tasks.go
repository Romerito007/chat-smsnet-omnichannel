package contracts

// AutomationInvoke is the Asynq payload to invoke the external flow for a new
// conversation. Automation is "slow, non-critical" work, so it runs out of band
// of the fast inbound response.
type AutomationInvoke struct {
	TenantID       string `json:"tenant_id"`
	IntegrationID  string `json:"integration_id"`
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
}

// AutomationDispatcher enqueues the (async) automation invocation. The
// implementation lives in infra and uses Asynq.
type AutomationDispatcher interface {
	Dispatch(payload AutomationInvoke) error
}

// DeliverTask is the Asynq payload for channel.deliver / channel.retry.
type DeliverTask struct {
	TenantID   string `json:"tenant_id"`
	DeliveryID string `json:"delivery_id"`
}

// DeliveryEnqueuer enqueues channel.deliver / channel.retry tasks. The
// implementation lives in infra and uses Asynq.
type DeliveryEnqueuer interface {
	// EnqueueDeliver schedules an immediate delivery attempt.
	EnqueueDeliver(payload DeliverTask) error
	// EnqueueRetry schedules a retry after delay (seconds).
	EnqueueRetry(payload DeliverTask, delaySeconds int) error
}
