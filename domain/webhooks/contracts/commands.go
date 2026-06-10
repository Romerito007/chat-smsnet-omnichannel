// Package contracts holds the webhooks service inputs, the delivery task payload
// and the outbound-client/enqueuer ports.
package contracts

// CreateSubscription is the input to register a webhook.
type CreateSubscription struct {
	Name   string
	URL    string
	Events []string
	Scopes []string
	// Secret is optional; when empty the service generates a strong one. It is
	// returned exactly once in the create response and never exposed again.
	Secret          string
	Enabled         *bool
	RateLimitPerMin int
}

// UpdateSubscription is the input to patch a webhook. Nil pointers and nil
// slices leave the existing value unchanged. The secret is immutable here (use
// rotate semantics if ever needed); it is never accepted on update.
type UpdateSubscription struct {
	Name            *string
	URL             *string
	Events          []string
	Scopes          []string
	Enabled         *bool
	RateLimitPerMin *int
}

// DeliverTask is the Asynq payload for webhook.deliver / webhook.retry. It
// carries only identifiers; the delivery record holds the signed payload.
type DeliverTask struct {
	TenantID   string `json:"tenant_id"`
	DeliveryID string `json:"delivery_id"`
}
