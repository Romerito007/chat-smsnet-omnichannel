// Package routing holds the request DTOs for the routing endpoints. Responses
// reuse the conversations DTOs (assignment results) and the routing RunResult.
package routing

import "github.com/romerito007/chat-smsnet-omnichannel/domain/routing/contracts"

// AssignRequest is the body of POST /v1/conversations/{id}/assign.
type AssignRequest struct {
	AgentID string `json:"agent_id"`
}

// ToCommand maps to the service command.
func (r AssignRequest) ToCommand() contracts.AssignCommand {
	return contracts.AssignCommand{AgentID: r.AgentID}
}

// TransferRequest is the body of POST /v1/conversations/{id}/transfer.
type TransferRequest struct {
	SectorID string `json:"sector_id"`
	AgentID  string `json:"agent_id"`
}

// ToCommand maps to the service command.
func (r TransferRequest) ToCommand() contracts.TransferCommand {
	return contracts.TransferCommand{SectorID: r.SectorID, AgentID: r.AgentID}
}

// EnqueueRequest is the body of POST /v1/conversations/{id}/enqueue.
type EnqueueRequest struct {
	QueueID string `json:"queue_id"`
}

// ToCommand maps to the service command.
func (r EnqueueRequest) ToCommand() contracts.EnqueueCommand {
	return contracts.EnqueueCommand{QueueID: r.QueueID}
}

// RunRequest is the body of POST /v1/routing/run. ConversationID is optional.
type RunRequest struct {
	ConversationID string `json:"conversation_id"`
}

// ToCommand maps to the service command.
func (r RunRequest) ToCommand() contracts.RunCommand {
	return contracts.RunCommand{ConversationID: r.ConversationID}
}
