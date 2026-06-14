package models

import "time"

// WebhookSubscription is the BSON document for a webhook subscription. The
// signing secret is stored encrypted.
type WebhookSubscription struct {
	Base             `bson:",inline"`
	Name             string   `bson:"name,omitempty"`
	URL              string   `bson:"url"`
	Events           []string `bson:"events"`
	Scopes           []string `bson:"scopes,omitempty"`
	EncryptedSecret  string   `bson:"encrypted_secret"`
	Enabled          bool     `bson:"enabled"`
	RateLimitPerMin  int      `bson:"rate_limit_per_minute,omitempty"`
	OwnedByChannelID string   `bson:"owned_by_channel_id,omitempty"`
	CreatedBy        string   `bson:"created_by,omitempty"`
}

// WebhookDelivery is the BSON document for a per-attempt delivery record.
type WebhookDelivery struct {
	ID          string     `bson:"_id"`
	TenantID    string     `bson:"tenant_id"`
	WebhookID   string     `bson:"webhook_id"`
	Event       string     `bson:"event"`
	Payload     []byte     `bson:"payload"`
	Status      string     `bson:"status"`
	Attempts    int        `bson:"attempts"`
	LastError   string     `bson:"last_error,omitempty"`
	NextRetryAt *time.Time `bson:"next_retry_at,omitempty"`
	CreatedAt   time.Time  `bson:"created_at"`
	UpdatedAt   time.Time  `bson:"updated_at"`
}
