package service

import (
	"context"
	"fmt"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/repository"
)

// defaultMaxAttempts bounds delivery retries before dead-lettering.
const defaultMaxAttempts = 6

// DeliveryService performs a single webhook delivery attempt, driving retries
// with exponential backoff and dead-lettering on exhaustion. It is invoked by
// the webhook.deliver / webhook.retry Asynq handlers.
type DeliveryService struct {
	subs        repository.SubscriptionRepository
	deliveries  repository.DeliveryRepository
	sender      contracts.Sender
	enqueuer    contracts.Enqueuer
	limiter     contracts.RateLimiter
	clock       shared.Clock
	maxAttempts int
}

// NewDeliveryService builds the service.
func NewDeliveryService(
	subs repository.SubscriptionRepository,
	deliveries repository.DeliveryRepository,
	sender contracts.Sender,
	enqueuer contracts.Enqueuer,
	limiter contracts.RateLimiter,
	clock shared.Clock,
) *DeliveryService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &DeliveryService{
		subs: subs, deliveries: deliveries, sender: sender, enqueuer: enqueuer,
		limiter: limiter, clock: clock, maxAttempts: defaultMaxAttempts,
	}
}

// Deliver attempts to deliver one record. It is idempotent: a terminal delivery
// is a no-op. On a 2xx it marks the delivery delivered; otherwise it retries
// with backoff up to maxAttempts, then dead-letters.
func (s *DeliveryService) Deliver(ctx context.Context, deliveryID string) error {
	delivery, err := s.deliveries.FindByID(ctx, deliveryID)
	if err != nil {
		return err
	}
	if delivery.Status.IsTerminal() {
		return nil // already delivered or dead → idempotent no-op
	}

	sub, err := s.subs.FindByID(ctx, delivery.WebhookID)
	if err != nil {
		// Subscription gone → nothing can deliver this; dead-letter it.
		if apperror.From(err).Code == apperror.CodeNotFound {
			return s.dead(ctx, delivery, "subscription no longer exists")
		}
		return err
	}
	if !sub.Enabled {
		return s.dead(ctx, delivery, "subscription disabled")
	}

	// Per-subscription rate limit. When exceeded, reschedule shortly without
	// consuming an attempt.
	if s.limiter != nil {
		if allowed, lerr := s.limiter.Allow(ctx, sub.ID, sub.RateLimitPerMin); lerr == nil && !allowed {
			return s.reschedule(ctx, delivery, 30, "rate limited")
		}
	}

	res, sendErr := s.sender.Send(ctx, sub, delivery)
	delivery.Attempts++
	if sendErr == nil && res.StatusCode >= 200 && res.StatusCode < 300 {
		return s.delivered(ctx, delivery)
	}

	reason := deliveryError(res, sendErr)
	return s.retryOrDead(ctx, delivery, reason)
}

// delivered marks the delivery successful.
func (s *DeliveryService) delivered(ctx context.Context, d *entity.WebhookDelivery) error {
	now := s.clock.Now()
	d.Status = entity.DeliveryDelivered
	d.LastError = ""
	d.NextRetryAt = nil
	d.UpdatedAt = now
	return s.deliveries.Update(ctx, d)
}

// retryOrDead schedules another attempt with backoff, or dead-letters when the
// attempt limit is reached.
func (s *DeliveryService) retryOrDead(ctx context.Context, d *entity.WebhookDelivery, reason string) error {
	if d.Attempts >= s.maxAttempts {
		return s.dead(ctx, d, reason)
	}
	backoff := backoffSeconds(d.Attempts)
	next := s.clock.Now().Add(durationSeconds(backoff))
	d.Status = entity.DeliveryRetrying
	d.LastError = reason
	d.NextRetryAt = &next
	d.UpdatedAt = s.clock.Now()
	if err := s.deliveries.Update(ctx, d); err != nil {
		return err
	}
	if s.enqueuer != nil {
		_ = s.enqueuer.EnqueueRetry(contracts.DeliverTask{TenantID: d.TenantID, DeliveryID: d.ID}, backoff)
	}
	return nil
}

// reschedule pushes the delivery out by delaySeconds without counting an
// attempt (used for rate limiting). The attempt increment done by the caller is
// rolled back here.
func (s *DeliveryService) reschedule(ctx context.Context, d *entity.WebhookDelivery, delaySeconds int, reason string) error {
	next := s.clock.Now().Add(durationSeconds(delaySeconds))
	d.Status = entity.DeliveryRetrying
	d.LastError = reason
	d.NextRetryAt = &next
	d.UpdatedAt = s.clock.Now()
	if err := s.deliveries.Update(ctx, d); err != nil {
		return err
	}
	if s.enqueuer != nil {
		_ = s.enqueuer.EnqueueRetry(contracts.DeliverTask{TenantID: d.TenantID, DeliveryID: d.ID}, delaySeconds)
	}
	return nil
}

// dead moves the delivery to the dead-letter state.
func (s *DeliveryService) dead(ctx context.Context, d *entity.WebhookDelivery, reason string) error {
	now := s.clock.Now()
	d.Status = entity.DeliveryDead
	d.LastError = reason
	d.NextRetryAt = nil
	d.UpdatedAt = now
	return s.deliveries.Update(ctx, d)
}

func deliveryError(res contracts.SendResult, err error) string {
	if err != nil {
		msg := err.Error()
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return msg
	}
	return fmt.Sprintf("endpoint returned status %d", res.StatusCode)
}
