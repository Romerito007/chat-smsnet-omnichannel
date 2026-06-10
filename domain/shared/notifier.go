package shared

import "context"

// NotifyInput is a request to notify a single recipient user. Producers (routing,
// conversations, sla, channels) build it; the notifications domain creates the
// in-app notification, publishes realtime and optionally sends email.
type NotifyInput struct {
	TenantID string
	UserID   string
	Type     string
	Title    string
	Body     string
	Link     string
}

// Notifier delivers a notification to a user. It is implemented by the
// notifications domain (enqueuing the notification.send job) and consulted by
// producers. Fire-and-forget: a notification failure must never break the
// operation that produced it. The default no-op drops notifications.
type Notifier interface {
	Notify(ctx context.Context, in NotifyInput)
}

// NoopNotifier discards notifications.
type NoopNotifier struct{}

// Notify implements Notifier.
func (NoopNotifier) Notify(context.Context, NotifyInput) {}
