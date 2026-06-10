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
