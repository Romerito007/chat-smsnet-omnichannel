// Package contracts holds the notifications job payloads, the email-sender /
// enqueuer ports and the service inputs.
package contracts

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/entity"
)

// RealtimeNotificationCreated is the realtime event published to the recipient
// when an in-app notification is created.
const RealtimeNotificationCreated = "notification.created"

// SendTask is the notification.send Asynq payload (one recipient).
type SendTask struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Link     string `json:"link"`
}

// EmailTask is the notification.email Asynq payload. It carries only the
// notification id; the worker loads it and the recipient's address.
type EmailTask struct {
	TenantID       string `json:"tenant_id"`
	NotificationID string `json:"notification_id"`
}

// EmailEnqueuer schedules the notification.email job.
type EmailEnqueuer interface {
	EnqueueEmail(task EmailTask) error
}

// EmailMessage is a rendered, privacy-safe email: it carries no sensitive body,
// only a title and a deep link back into the app.
type EmailMessage struct {
	To      string
	Subject string
	Link    string
	// Preview is a short, non-sensitive line (e.g. the notification type label).
	Preview string
}

// EmailSender delivers an email. Implemented in infra/email.
type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

// UpdatePreferences is the input to patch a user's email preferences. Only the
// provided types are changed.
type UpdatePreferences struct {
	EmailByType map[entity.Type]bool
}
