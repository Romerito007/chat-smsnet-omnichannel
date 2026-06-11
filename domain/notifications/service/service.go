// Package service holds the notifications business logic: the in-app + email
// delivery handlers (notification.send / notification.email) and the user-facing
// inbox and preferences.
package service

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service implements the notifications use cases.
type Service struct {
	notifications repository.NotificationRepository
	preferences   repository.PreferencesRepository
	users         iamrepo.UserRepository
	publisher     shared.EventPublisher
	emailEnqueuer contracts.EmailEnqueuer
	emailSender   contracts.EmailSender
	clock         shared.Clock
}

// NewService builds the service.
func NewService(
	notifications repository.NotificationRepository,
	preferences repository.PreferencesRepository,
	users iamrepo.UserRepository,
	publisher shared.EventPublisher,
	emailEnqueuer contracts.EmailEnqueuer,
	emailSender contracts.EmailSender,
	clock shared.Clock,
) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if publisher == nil {
		publisher = shared.NoopPublisher{}
	}
	return &Service{
		notifications: notifications, preferences: preferences, users: users,
		publisher: publisher, emailEnqueuer: emailEnqueuer, emailSender: emailSender, clock: clock,
	}
}

// ── handlers (Asynq) ─────────────────────────────────────────────────────────

// Send is the notification.send handler: it always creates the in-app
// notification and publishes realtime notification.created to the recipient, then
// enqueues an email when the recipient's preference enables it for the type.
func (s *Service) Send(ctx context.Context, task contracts.SendTask) error {
	if task.UserID == "" {
		return nil
	}
	ntype := entity.Type(task.Type)
	if !entity.IsValidType(ntype) {
		return nil // unknown type → drop
	}

	now := s.clock.Now()
	n := &entity.Notification{
		ID:        shared.NewID(),
		TenantID:  task.TenantID,
		UserID:    task.UserID,
		Type:      ntype,
		Title:     task.Title,
		Body:      task.Body,
		Link:      task.Link,
		CreatedAt: now,
	}
	if err := s.notifications.Create(ctx, n); err != nil {
		return err
	}

	// In-app realtime to the recipient's personal topic.
	_ = s.publisher.Publish(ctx, shared.TopicUser(task.TenantID, task.UserID),
		contracts.RealtimeNotificationCreated, newPayload(n))

	// Email (optional, per type + user preference). Never blocks in-app delivery.
	prefs, _ := s.preferences.FindByUser(ctx, task.UserID)
	if prefs.EmailEnabled(ntype) && s.emailEnqueuer != nil {
		_ = s.emailEnqueuer.EnqueueEmail(contracts.EmailTask{TenantID: task.TenantID, NotificationID: n.ID})
	}
	return nil
}

// SendEmail is the notification.email handler. It renders a privacy-safe email
// (subject + link only — no sensitive body) and sends it.
func (s *Service) SendEmail(ctx context.Context, task contracts.EmailTask) error {
	if s.emailSender == nil {
		return nil
	}
	n, err := s.notifications.FindByID(ctx, task.NotificationID)
	if err != nil {
		return err
	}
	user, err := s.users.FindByID(ctx, n.UserID)
	if err != nil {
		return err
	}
	if user.Email == "" {
		return nil
	}
	// Privacy: only the title (a non-sensitive label) and the deep link leave the
	// system; the notification body is never placed in the email.
	return s.emailSender.Send(ctx, contracts.EmailMessage{
		To:      user.Email,
		Subject: n.Title,
		Link:    n.Link,
		Preview: string(n.Type),
	})
}

// Cleanup removes read notifications created at or before the cutoff for the
// current tenant. Idempotent. Run by the notifications.cleanup job.
func (s *Service) Cleanup(ctx context.Context, before time.Time) (int, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return 0, err
	}
	return s.notifications.DeleteReadBefore(ctx, before)
}

// ── user-facing ──────────────────────────────────────────────────────────────

// List returns the authenticated user's notifications.
func (s *Service) List(ctx context.Context, unreadOnly bool, page shared.PageRequest) ([]*entity.Notification, error) {
	userID, err := s.actorUser(ctx)
	if err != nil {
		return nil, err
	}
	return s.notifications.ListByUser(ctx, userID, unreadOnly, page.Normalize())
}

// MarkRead marks one of the user's notifications read.
func (s *Service) MarkRead(ctx context.Context, id string) error {
	userID, err := s.actorUser(ctx)
	if err != nil {
		return err
	}
	return s.notifications.MarkRead(ctx, id, userID, s.clock.Now())
}

// MarkAllRead marks all of the user's notifications read, returning the count.
func (s *Service) MarkAllRead(ctx context.Context) (int, error) {
	userID, err := s.actorUser(ctx)
	if err != nil {
		return 0, err
	}
	return s.notifications.MarkAllRead(ctx, userID, s.clock.Now())
}

// Preferences returns the authenticated user's effective email preferences.
func (s *Service) Preferences(ctx context.Context) (*entity.Preferences, error) {
	userID, err := s.actorUser(ctx)
	if err != nil {
		return nil, err
	}
	tenantID, _ := shared.RequireTenant(ctx)
	prefs, err := s.preferences.FindByUser(ctx, userID)
	if err != nil && apperror.From(err).Code != apperror.CodeNotFound {
		return nil, err
	}
	if prefs == nil {
		prefs = &entity.Preferences{TenantID: tenantID, UserID: userID}
	}
	return prefs, nil
}

// UpdatePreferences patches the user's per-type email preferences.
func (s *Service) UpdatePreferences(ctx context.Context, cmd contracts.UpdatePreferences) (*entity.Preferences, error) {
	userID, err := s.actorUser(ctx)
	if err != nil {
		return nil, err
	}
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	prefs, ferr := s.preferences.FindByUser(ctx, userID)
	if ferr != nil && apperror.From(ferr).Code != apperror.CodeNotFound {
		return nil, ferr
	}
	if prefs == nil {
		prefs = &entity.Preferences{TenantID: tenantID, UserID: userID, EmailByType: map[entity.Type]bool{}}
	}
	if prefs.EmailByType == nil {
		prefs.EmailByType = map[entity.Type]bool{}
	}
	for t, v := range cmd.EmailByType {
		if entity.IsValidType(t) {
			prefs.EmailByType[t] = v
		}
	}
	prefs.UpdatedAt = s.clock.Now()
	if err := s.preferences.Upsert(ctx, prefs); err != nil {
		return nil, err
	}
	return prefs, nil
}

func (s *Service) actorUser(ctx context.Context) (string, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return "", err
	}
	ac, ok := authz.FromContext(ctx)
	if !ok || ac.UserID == "" {
		return "", apperror.Unauthorized("authentication required")
	}
	return ac.UserID, nil
}
