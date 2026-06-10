package contracts

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
)

// Sender delivers one signed webhook request to a subscription's URL. The
// implementation (infra/webhooks) computes the HMAC-SHA256 signature and sets
// the X-Webhook-* headers. It returns the HTTP status code on a completed
// request, or an error when the request could not be completed.
type Sender interface {
	Send(ctx context.Context, sub *entity.WebhookSubscription, delivery *entity.WebhookDelivery) (SendResult, error)
}

// SendResult is the outcome of a delivery attempt.
type SendResult struct {
	StatusCode int
}

// Enqueuer schedules webhook delivery jobs on the Asynq `webhooks` queue.
type Enqueuer interface {
	EnqueueDeliver(task DeliverTask) error
	EnqueueRetry(task DeliverTask, delaySeconds int) error
}

// RateLimiter caps the per-subscription delivery rate per minute.
type RateLimiter interface {
	// Allow reports whether a delivery to the webhook may proceed now. limitPerMin
	// of 0 means unlimited.
	Allow(ctx context.Context, webhookID string, limitPerMin int) (bool, error)
}
