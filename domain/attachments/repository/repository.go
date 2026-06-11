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
}
