// Package repository declares the attachment persistence contract (tenant-scoped
// from context).
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/entity"
)

// Repository persists attachments.
type Repository interface {
	Create(ctx context.Context, a *entity.Attachment) error
	Update(ctx context.Context, a *entity.Attachment) error
	FindByID(ctx context.Context, id string) (*entity.Attachment, error)
	// FindByIDs batch-loads attachments by id within the tenant (order is not
	// guaranteed; missing ids are simply absent). Used to hydrate message
	// attachments at the read boundary without an N+1.
	FindByIDs(ctx context.Context, ids []string) ([]*entity.Attachment, error)
}
