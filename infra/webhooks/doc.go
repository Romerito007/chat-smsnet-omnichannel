// Package webhooks holds the outbound webhook delivery client with HMAC
// signing. Delivery and retries run on the Asynq `webhooks` queue
// (webhook.deliver / webhook.retry); terminal failures go to the Asynq
// dead-letter.
//
// Placeholder in the foundation.
package webhooks
