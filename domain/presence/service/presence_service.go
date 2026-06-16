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

// List returns agent presence with a freshly recomputed load. When sectorID is
// non-empty it returns only the agents linked to that sector. Load is derived in
// ONE aggregation across the tenant (no count-per-agent N+1).
func (s *Service) List(ctx context.Context, sectorID string) ([]*entity.AgentPresence, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	items, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	if sectorID != "" {
		users, err := s.users.ListBySector(ctx, sectorID)
		if err != nil {
			return nil, err
		}
		allowed := make(map[string]struct{}, len(users))
		for _, u := range users {
			allowed[u.ID] = struct{}{}
		}
		filtered := items[:0]
		for _, p := range items {
			if _, ok := allowed[p.UserID]; ok {
				filtered = append(filtered, p)
			}
		}
		items = filtered
	}
	loads, err := s.load.OpenAssignedLoads(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range items {
		p.CurrentLoad = loads[p.UserID]
	}
	return items, nil
}

// Touch renews the liveness TTL for an agent's presence. The WS transport calls
// it on connect and on every heartbeat while the socket is open. It NEVER changes
// the stored status, so it neither promotes a freshly connected socket to "online"
// (availability is an explicit choice, not a side effect of connecting) nor
// overrides a deliberate "offline": it only keeps an already-declared status alive
// while the socket is. A missing record is a no-op. Best-effort.
func (s *Service) Touch(ctx context.Context, userID string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	return s.store.Touch(ctx, userID)
}

// Vanished records that an agent's session disappeared — a graceful WS disconnect
// or a TTL expiry caught by the keyspace watcher. It drops the presence record and
// the roster entry, then publishes the offline event (presence board + the agent's
// own user room) so open dashboards update live instead of waiting for a refetch.
// Idempotent: a redundant call (e.g. the expiry firing after a graceful close
// already removed the key) simply republishes offline.
func (s *Service) Vanished(ctx context.Context, userID string) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	if err := s.store.Remove(ctx, userID); err != nil {
		return err
	}
	offline := &entity.AgentPresence{
		TenantID:   tenantID,
		UserID:     userID,
		Status:     entity.StatusOffline,
		LastSeenAt: s.clock.Now(),
	}
	event := contracts.NewPresenceChanged(offline)
	_ = s.publisher.Publish(ctx, shared.TopicPresence(tenantID), contracts.EventPresenceChanged, event)
	_ = s.publisher.Publish(ctx, shared.TopicUser(tenantID, userID), contracts.EventPresenceChanged, event)
	return nil
}

// currentStatus returns the stored status, or offline when no record exists.
func (s *Service) currentStatus(ctx context.Context, userID string) entity.Status {
	cur, err := s.store.Get(ctx, userID)
	if err != nil || cur == nil {
		return entity.StatusOffline
	}
	return cur.Status
}
