// Package webhooks holds the request/response DTOs for the webhook endpoints.
// The signing secret is returned exactly once (on creation) and never again;
// subsequent responses only report whether a secret is set.
package webhooks

import (
	"encoding/json"
	"time"

	wcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/contracts"
	wentity "github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
)

// CreateRequest is the body of POST /v1/webhooks.
type CreateRequest struct {
	Name            string   `json:"name"`
	URL             string   `json:"url"`
	Events          []string `json:"events"`
	Scopes          []string `json:"scopes"`
	Secret          string   `json:"secret"`
	Enabled         *bool    `json:"enabled"`
	RateLimitPerMin int      `json:"rate_limit_per_minute"`
}

// ToCommand maps to the service command.
func (r CreateRequest) ToCommand() wcontracts.CreateSubscription {
	return wcontracts.CreateSubscription{
		Name:            r.Name,
		URL:             r.URL,
		Events:          r.Events,
		Scopes:          r.Scopes,
		Secret:          r.Secret,
		Enabled:         r.Enabled,
		RateLimitPerMin: r.RateLimitPerMin,
	}
}

// UpdateRequest is the body of PATCH /v1/webhooks/{id}.
type UpdateRequest struct {
	Name            *string  `json:"name"`
	URL             *string  `json:"url"`
	Events          []string `json:"events"`
	Scopes          []string `json:"scopes"`
	Enabled         *bool    `json:"enabled"`
	RateLimitPerMin *int     `json:"rate_limit_per_minute"`
}

// ToCommand maps to the service command.
func (r UpdateRequest) ToCommand() wcontracts.UpdateSubscription {
	return wcontracts.UpdateSubscription{
		Name:            r.Name,
		URL:             r.URL,
		Events:          r.Events,
		Scopes:          r.Scopes,
		Enabled:         r.Enabled,
		RateLimitPerMin: r.RateLimitPerMin,
	}
}

// SubscriptionResponse is the public representation (secret never included).
type SubscriptionResponse struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenant_id"`
	Name            string    `json:"name,omitempty"`
	URL             string    `json:"url"`
	Events          []string  `json:"events"`
	Scopes          []string  `json:"scopes,omitempty"`
	HasSecret       bool      `json:"has_secret"`
	Enabled         bool      `json:"enabled"`
	RateLimitPerMin int       `json:"rate_limit_per_minute"`
	CreatedBy       string    `json:"created_by,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// NewSubscriptionResponse maps an entity (without exposing the secret).
func NewSubscriptionResponse(s *wentity.WebhookSubscription) SubscriptionResponse {
	return SubscriptionResponse{
		ID:              s.ID,
		TenantID:        s.TenantID,
		Name:            s.Name,
		URL:             s.URL,
		Events:          s.Events,
		Scopes:          s.Scopes,
		HasSecret:       s.Secret != "",
		Enabled:         s.Enabled,
		RateLimitPerMin: s.RateLimitPerMin,
		CreatedBy:       s.CreatedBy,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}
}

// NewSubscriptionResponses maps a slice.
func NewSubscriptionResponses(items []*wentity.WebhookSubscription) []SubscriptionResponse {
	out := make([]SubscriptionResponse, 0, len(items))
	for _, s := range items {
		out = append(out, NewSubscriptionResponse(s))
	}
	return out
}

// CreatedResponse is the create response: the public view plus the plaintext
// secret, exposed exactly once so the integrator can store it.
type CreatedResponse struct {
	SubscriptionResponse
	Secret string `json:"secret"`
}

// NewCreatedResponse maps a freshly created subscription.
func NewCreatedResponse(s *wentity.WebhookSubscription) CreatedResponse {
	return CreatedResponse{SubscriptionResponse: NewSubscriptionResponse(s), Secret: s.Secret}
}

// DeliveryResponse is the public representation of a delivery record.
type DeliveryResponse struct {
	ID          string          `json:"id"`
	WebhookID   string          `json:"webhook_id"`
	Event       string          `json:"event"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	Status      string          `json:"status"`
	Attempts    int             `json:"attempts"`
	LastError   string          `json:"last_error,omitempty"`
	NextRetryAt *time.Time      `json:"next_retry_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// NewDeliveryResponse maps a delivery entity.
func NewDeliveryResponse(d *wentity.WebhookDelivery) DeliveryResponse {
	return DeliveryResponse{
		ID:          d.ID,
		WebhookID:   d.WebhookID,
		Event:       d.Event,
		Payload:     json.RawMessage(d.Payload),
		Status:      string(d.Status),
		Attempts:    d.Attempts,
		LastError:   d.LastError,
		NextRetryAt: d.NextRetryAt,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

// NewDeliveryResponses maps a slice.
func NewDeliveryResponses(items []*wentity.WebhookDelivery) []DeliveryResponse {
	out := make([]DeliveryResponse, 0, len(items))
	for _, d := range items {
		out = append(out, NewDeliveryResponse(d))
	}
	return out
}
