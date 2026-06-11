package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/repository"
)

// TestResult is the outcome of POST /v1/webhooks/{id}/test.
type TestResult struct {
	DeliveryID string `json:"delivery_id"`
	OK         bool   `json:"ok"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// SubscriptionService manages webhook subscriptions and synchronous test
// deliveries.
type SubscriptionService struct {
	subs       repository.SubscriptionRepository
	deliveries repository.DeliveryRepository
	sender     contracts.Sender
	clock      shared.Clock
	auditor    shared.Auditor
}

// NewSubscriptionService builds the service.
func NewSubscriptionService(
	subs repository.SubscriptionRepository,
	deliveries repository.DeliveryRepository,
	sender contracts.Sender,
	clock shared.Clock,
) *SubscriptionService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &SubscriptionService{subs: subs, deliveries: deliveries, sender: sender, clock: clock, auditor: shared.NoopAuditor{}}
}

// SetAuditor wires the audit trail. Optional: when unset, webhook changes are not
// audited.
func (s *SubscriptionService) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// Create registers a webhook. The returned entity carries the plaintext secret
// so the controller can expose it exactly once; it is never returned again.
func (s *SubscriptionService) Create(ctx context.Context, cmd contracts.CreateSubscription) (*entity.WebhookSubscription, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateURL(cmd.URL); err != nil {
		return nil, err
	}
	events, err := normalizeEvents(cmd.Events)
	if err != nil {
		return nil, err
	}

	secret := strings.TrimSpace(cmd.Secret)
	if secret == "" {
		secret = "whsec_" + randomToken(32)
	}
	enabled := true
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	}
	now := s.clock.Now()
	createdBy := ""
	if ac, ok := authz.FromContext(ctx); ok {
		createdBy = ac.UserID
	}

	sub := &entity.WebhookSubscription{
		ID:              shared.NewID(),
		TenantID:        tenantID,
		Name:            strings.TrimSpace(cmd.Name),
		URL:             strings.TrimSpace(cmd.URL),
		Events:          events,
		Scopes:          cmd.Scopes,
		Secret:          secret,
		Enabled:         enabled,
		RateLimitPerMin: cmd.RateLimitPerMin,
		CreatedBy:       createdBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.subs.Create(ctx, sub); err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "webhook.created", ResourceType: "webhook", ResourceID: sub.ID,
		Data: map[string]any{"url": sub.URL, "events": sub.Events},
	})
	return sub, nil
}

// List returns the tenant's webhooks (secret never populated for listing).
func (s *SubscriptionService) List(ctx context.Context, page shared.PageRequest) ([]*entity.WebhookSubscription, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.subs.List(ctx, page.Normalize())
}

// Get returns one webhook by id.
func (s *SubscriptionService) Get(ctx context.Context, id string) (*entity.WebhookSubscription, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.subs.FindByID(ctx, id)
}

// Update patches a webhook. The secret is immutable here.
func (s *SubscriptionService) Update(ctx context.Context, id string, cmd contracts.UpdateSubscription) (*entity.WebhookSubscription, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	sub, err := s.subs.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		sub.Name = strings.TrimSpace(*cmd.Name)
	}
	if cmd.URL != nil {
		if err := validateURL(*cmd.URL); err != nil {
			return nil, err
		}
		sub.URL = strings.TrimSpace(*cmd.URL)
	}
	if cmd.Events != nil {
		events, err := normalizeEvents(cmd.Events)
		if err != nil {
			return nil, err
		}
		sub.Events = events
	}
	if cmd.Scopes != nil {
		sub.Scopes = cmd.Scopes
	}
	if cmd.Enabled != nil {
		sub.Enabled = *cmd.Enabled
	}
	if cmd.RateLimitPerMin != nil {
		sub.RateLimitPerMin = *cmd.RateLimitPerMin
	}
	sub.UpdatedAt = s.clock.Now()
	if err := s.subs.Update(ctx, sub); err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "webhook.updated", ResourceType: "webhook", ResourceID: sub.ID,
		Data: map[string]any{"url": sub.URL, "enabled": sub.Enabled},
	})
	return sub, nil
}

// Delete removes a webhook.
func (s *SubscriptionService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	// Ensure it exists (and belongs to the tenant) before deleting.
	if _, err := s.subs.FindByID(ctx, id); err != nil {
		return err
	}
	if err := s.subs.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		Action: "webhook.deleted", ResourceType: "webhook", ResourceID: id,
	})
	return nil
}

// Test sends a signed test event to the webhook synchronously and records the
// delivery, returning the immediate outcome so the integrator can verify HMAC.
func (s *SubscriptionService) Test(ctx context.Context, id string) (TestResult, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return TestResult{}, err
	}
	sub, err := s.subs.FindByID(ctx, id)
	if err != nil {
		return TestResult{}, err
	}

	now := s.clock.Now()
	deliveryID := shared.NewID()
	body, err := buildEnvelope(deliveryID, "webhook.test", now, map[string]any{
		"message": "This is a test webhook delivery.",
		"webhook": sub.ID,
	})
	if err != nil {
		return TestResult{}, apperror.Internal("build test payload").Wrap(err)
	}
	delivery := &entity.WebhookDelivery{
		ID:        deliveryID,
		TenantID:  sub.TenantID,
		WebhookID: sub.ID,
		Event:     "webhook.test",
		Payload:   body,
		Status:    entity.DeliveryPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	res, sendErr := s.sender.Send(ctx, sub, delivery)
	delivery.Attempts = 1
	delivery.UpdatedAt = s.clock.Now()
	out := TestResult{DeliveryID: deliveryID, StatusCode: res.StatusCode}
	if sendErr == nil && res.StatusCode >= 200 && res.StatusCode < 300 {
		delivery.Status = entity.DeliveryDelivered
		out.OK = true
	} else {
		delivery.Status = entity.DeliveryFailed
		delivery.LastError = deliveryError(res, sendErr)
		out.Error = "the endpoint did not accept the test delivery"
	}
	_ = s.deliveries.Create(ctx, delivery)
	return out, nil
}

// ListDeliveries returns a webhook's delivery history (newest first).
func (s *SubscriptionService) ListDeliveries(ctx context.Context, webhookID string, page shared.PageRequest) ([]*entity.WebhookDelivery, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	// Ensure the webhook belongs to the tenant before listing its deliveries.
	if _, err := s.subs.FindByID(ctx, webhookID); err != nil {
		return nil, err
	}
	return s.deliveries.ListByWebhook(ctx, webhookID, page.Normalize())
}

// ── helpers ──────────────────────────────────────────────────────────────────

func normalizeEvents(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, apperror.Validation("at least one event is required").
			WithDetails(map[string]any{"events": "is required"})
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, e := range in {
		e = strings.TrimSpace(e)
		if !entity.IsSupportedEvent(e) {
			return nil, apperror.Validation("unsupported event: " + e).
				WithDetails(map[string]any{"events": "unsupported event " + e})
		}
		if _, dup := seen[e]; dup {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out, nil
}

func validateURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" && u.Scheme != "http" || u.Host == "" {
		return apperror.Validation("url must be a valid http(s) URL").
			WithDetails(map[string]any{"url": "must be a valid http(s) URL"})
	}
	return nil
}

func randomToken(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
