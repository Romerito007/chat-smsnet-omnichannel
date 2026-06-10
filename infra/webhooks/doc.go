// Package webhooks holds the outbound webhook delivery client with HMAC-SHA256
// signing, the Asynq enqueuer (webhook.deliver / webhook.retry on the
// `webhooks` queue) and the per-subscription Redis rate limiter. Terminal
// failures are dead-lettered by the domain delivery service after the retry
// limit.
package webhooks
