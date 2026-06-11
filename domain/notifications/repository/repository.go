// Package repository declares the notifications persistence contracts.
package repository

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// NotificationRepository persists notifications. All operations are scoped to
// the tenant (from context) and a recipient user id.
type NotificationRepository interface {
	Create(ctx context.Context, n *entity.Notification) error
	FindByID(ctx context.Context, id string) (*entity.Notification, error)
	// ListByUser returns the user's notifications, newest first (keyset). When
	// unreadOnly is true only unread ones are returned.
	ListByUser(ctx context.Context, userID string, unreadOnly bool, page shared.PageRequest) ([]*entity.Notification, error)
	// MarkRead marks one notification read; it must belong to the user.
	MarkRead(ctx context.Context, id, userID string, at time.Time) error
	// MarkAllRead marks every unread notification of the user read.
	MarkAllRead(ctx context.Context, userID string, at time.Time) (int, error)
	// DeleteReadBefore removes read notifications created at or before the cutoff
	// for the tenant. Used by the notifications.cleanup job. Returns the count.
	DeleteReadBefore(ctx context.Context, before time.Time) (int, error)
}

// PreferencesRepository persists per-user email preferences.
type PreferencesRepository interface {
	FindByUser(ctx context.Context, userID string) (*entity.Preferences, error)
	Upsert(ctx context.Context, p *entity.Preferences) error
}
