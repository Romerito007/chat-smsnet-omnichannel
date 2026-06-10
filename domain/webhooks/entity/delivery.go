package entity

import "time"

// DeliveryStatus tracks one webhook delivery through its retry lifecycle.
type DeliveryStatus string

const (
	// DeliveryPending is a delivery awaiting its first attempt.
	DeliveryPending DeliveryStatus = "pending"
	// DeliveryDelivered is a delivery the endpoint acknowledged (2xx).
	DeliveryDelivered DeliveryStatus = "delivered"
	// DeliveryRetrying is a failed delivery scheduled for another attempt.
	DeliveryRetrying DeliveryStatus = "retrying"
	// DeliveryFailed is a delivery that failed an attempt (transient state used
	// before the next retry is scheduled or it is moved to dead).
	DeliveryFailed DeliveryStatus = "failed"
	// DeliveryDead is a delivery that exhausted its retries (dead-letter).
	DeliveryDead DeliveryStatus = "dead"
)

// IsTerminal reports whether the delivery has reached a final state.
func (s DeliveryStatus) IsTerminal() bool {
	return s == DeliveryDelivered || s == DeliveryDead
}

// WebhookDelivery is the per-attempt record of delivering one event to one
// subscription. The payload is the exact JSON body that was (or will be) signed
// and sent, kept so deliveries can be inspected and replayed.
type WebhookDelivery struct {
	ID          string
	TenantID    string
	WebhookID   string
	Event       string
	Payload     []byte
	Status      DeliveryStatus
	Attempts    int
	LastError   string
	NextRetryAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
