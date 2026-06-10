// Package queues holds the request/response DTOs for the queue endpoints.
package queues

import (
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/queues/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/queues/entity"
)

// CreateQueueRequest is the body of POST /v1/queues.
type CreateQueueRequest struct {
	SectorID       string `json:"sector_id"`
	Name           string `json:"name"`
	Strategy       string `json:"strategy"`
	MaxWaitSeconds int    `json:"max_wait_seconds"`
	Enabled        *bool  `json:"enabled"`
}

// ToCommand maps the request to the service command.
func (r CreateQueueRequest) ToCommand() contracts.CreateQueue {
	return contracts.CreateQueue{
		SectorID:       r.SectorID,
		Name:           r.Name,
		Strategy:       entity.Strategy(r.Strategy),
		MaxWaitSeconds: r.MaxWaitSeconds,
		Enabled:        r.Enabled,
	}
}

// UpdateQueueRequest is the body of PATCH /v1/queues/{id}.
type UpdateQueueRequest struct {
	Name           *string `json:"name"`
	Strategy       *string `json:"strategy"`
	MaxWaitSeconds *int    `json:"max_wait_seconds"`
	Enabled        *bool   `json:"enabled"`
}

// ToCommand maps the request to the service command.
func (r UpdateQueueRequest) ToCommand() contracts.UpdateQueue {
	cmd := contracts.UpdateQueue{
		Name:           r.Name,
		MaxWaitSeconds: r.MaxWaitSeconds,
		Enabled:        r.Enabled,
	}
	if r.Strategy != nil {
		st := entity.Strategy(*r.Strategy)
		cmd.Strategy = &st
	}
	return cmd
}

// QueueResponse is the public representation of a queue.
type QueueResponse struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	SectorID       string    `json:"sector_id"`
	Name           string    `json:"name"`
	Strategy       string    `json:"strategy"`
	MaxWaitSeconds int       `json:"max_wait_seconds"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// NewQueueResponse maps a queue entity to its DTO.
func NewQueueResponse(q *entity.Queue) QueueResponse {
	return QueueResponse{
		ID:             q.ID,
		TenantID:       q.TenantID,
		SectorID:       q.SectorID,
		Name:           q.Name,
		Strategy:       string(q.Strategy),
		MaxWaitSeconds: q.MaxWaitSeconds,
		Enabled:        q.Enabled,
		CreatedAt:      q.CreatedAt,
		UpdatedAt:      q.UpdatedAt,
	}
}

// NewQueueResponses maps a slice of queues.
func NewQueueResponses(queues []*entity.Queue) []QueueResponse {
	out := make([]QueueResponse, len(queues))
	for i, q := range queues {
		out[i] = NewQueueResponse(q)
	}
	return out
}
