// Package repository declares the queue persistence contract.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/queues/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// QueueRepository persists queues within a tenant (scope from context).
type QueueRepository interface {
	Create(ctx context.Context, q *entity.Queue) error
	Update(ctx context.Context, q *entity.Queue) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.Queue, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.Queue, error)
}
