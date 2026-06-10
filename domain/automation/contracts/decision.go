// Package contracts holds the automation service inputs/outputs, the external
// flow decision vocabulary and the integration ports.
package contracts

// DecisionType is an action the external flow asks the chat to perform.
type DecisionType string

const (
	DecisionSendMessage       DecisionType = "send_message"
	DecisionAssignSector      DecisionType = "assign_sector"
	DecisionAssignAgent       DecisionType = "assign_agent"
	DecisionEnqueue           DecisionType = "enqueue"
	DecisionCloseConversation DecisionType = "close_conversation"
	DecisionAddTag            DecisionType = "add_tag"
	DecisionSetPriority       DecisionType = "set_priority"
	DecisionRequestHuman      DecisionType = "request_human"
	DecisionCallWebhook       DecisionType = "call_webhook"
	DecisionNoAction          DecisionType = "no_action"
)

// Decision is the flow's instruction. Only the fields relevant to Type are used.
type Decision struct {
	Type       DecisionType `json:"type"`
	Text       string       `json:"text,omitempty"`        // send_message
	SectorID   string       `json:"sector_id,omitempty"`   // assign_sector / request_human / enqueue
	AgentID    string       `json:"agent_id,omitempty"`    // assign_agent
	QueueID    string       `json:"queue_id,omitempty"`    // enqueue / request_human
	Tag        string       `json:"tag,omitempty"`         // add_tag
	Priority   string       `json:"priority,omitempty"`    // set_priority
	ReasonID   string       `json:"reason_id,omitempty"`   // close_conversation
	WebhookURL string       `json:"webhook_url,omitempty"` // call_webhook
}
