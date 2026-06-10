// Package service holds the presence business logic: status changes (with
// authorization and the availability rule), load derivation and realtime events.
package service

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/presence/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/presence/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/presence/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Service manages agent presence.
type Service struct {
	store     repository.PresenceStore
	load      repository.LoadCounter
	users     iamrepo.UserRepository
	publisher shared.EventPublisher
	clock     shared.Clock
}

// New builds the service.
func New(
	store repository.PresenceStore,
	load repository.LoadCounter,
	users iamrepo.UserRepository,
	publisher shared.EventPublisher,
	clock shared.Clock,
) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if publisher == nil {
		publisher = shared.NoopPublisher{}
	}
	return &Service{store: store, load: load, users: users, publisher: publisher, clock: clock}
}

// SetStatus changes an agent's status and publishes the realtime event.
//
// Authorization: a user may only change their own status, unless they hold
// user.manage (supervisor/admin). The "available" status additionally requires
// the agent to be online and to have at least one linked sector.
func (s *Service) SetStatus(ctx context.Context, targetUserID string, status entity.Status) (*entity.AgentPresence, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return nil, apperror.Unauthorized("authentication required")
	}

	target := targetUserID
	if target == "" {
		target = ac.UserID
	}
	if target != ac.UserID && !ac.Has(authz.UserManage) {
		return nil, apperror.Forbidden("cannot change another agent's status")
	}
	if !status.Valid() {
		return nil, apperror.Validation("invalid status").
			WithDetails(map[string]any{"status": "unknown value"})
	}

	user, err := s.users.FindByID(ctx, target)
	if err != nil {
		return nil, err
	}

	load, err := s.load.CountOpenAssigned(ctx, target)
	if err != nil {
		return nil, err
	}

	if status == entity.StatusAvailable {
		if len(user.SectorIDs) == 0 {
			return nil, apperror.Validation("available requires at least one linked sector").
				WithDetails(map[string]any{"sector_ids": "required to become available"})
		}
		current := s.currentStatus(ctx, target)
		if current != entity.StatusOnline && current != entity.StatusAvailable {
			return nil, apperror.Conflict("agent must be online before becoming available")
		}
	}

	presence := &entity.AgentPresence{
		TenantID:           tenantID,
		UserID:             target,
		Status:             status,
		CurrentLoad:        load,
		MaxConcurrentChats: user.MaxConcurrentChats,
		LastSeenAt:         s.clock.Now(),
	}
	if err := s.store.Save(ctx, presence); err != nil {
		return nil, err
	}

	// Best-effort realtime fan-out; a transport hiccup must not fail the request.
	event := contracts.NewPresenceChanged(presence)
	_ = s.publisher.Publish(ctx, shared.TopicPresence(tenantID), contracts.EventPresenceChanged, event)
	_ = s.publisher.Publish(ctx, shared.TopicUser(tenantID, target), contracts.EventPresenceChanged, event)

	return presence, nil
}

// List returns every agent's presence with a freshly recomputed load.
func (s *Service) List(ctx context.Context) ([]*entity.AgentPresence, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	items, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range items {
		if load, err := s.load.CountOpenAssigned(ctx, p.UserID); err == nil {
			p.CurrentLoad = load
		}
	}
	return items, nil
}

// currentStatus returns the stored status, or offline when no record exists.
func (s *Service) currentStatus(ctx context.Context, userID string) entity.Status {
	cur, err := s.store.Get(ctx, userID)
	if err != nil || cur == nil {
		return entity.StatusOffline
	}
	return cur.Status
}
