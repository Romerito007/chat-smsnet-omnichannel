package contracts

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
