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

// SetStatus sets an agent's DURABLE manual availability (online|away|offline) and
// publishes the recomputed effective status. The availability is persisted on the
// user document, so it sticks across logout/reconnect/tab/machine and the Redis TTL —
// only the agent changes it. Authorization: self, or user.manage for another agent.
func (s *Service) SetStatus(ctx context.Context, targetUserID string, status entity.Status) (*entity.AgentPresence, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	target, err := s.authorizeTarget(ctx, targetUserID)
	if err != nil {
		return nil, err
	}
	availability := string(status)
	if !validAvailability(availability) {
		return nil, apperror.Validation("invalid status").
			WithDetails(map[string]any{"status": "must be online, away or offline"})
	}
	if err := s.users.SetPresenceSettings(ctx, target, &availability, nil); err != nil {
		return nil, err
	}
	return s.recomputeAndPublish(ctx, target)
}

// SetAutoOffline sets the per-agent auto-offline toggle (durable) and publishes the
// recomputed effective status. Authorization: self, or user.manage.
func (s *Service) SetAutoOffline(ctx context.Context, targetUserID string, enabled bool) (*entity.AgentPresence, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	target, err := s.authorizeTarget(ctx, targetUserID)
	if err != nil {
		return nil, err
	}
	if err := s.users.SetPresenceSettings(ctx, target, nil, &enabled); err != nil {
		return nil, err
	}
	return s.recomputeAndPublish(ctx, target)
}

// Connected records a new live socket for the agent. When it is the agent's FIRST
// live socket, the effective status is recomputed (an availability=online agent
// becomes online) and published.
func (s *Service) Connected(ctx context.Context, userID, clientID string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	becameLive, err := s.store.Connect(ctx, userID, clientID)
	if err != nil {
		return err
	}
	if becameLive {
		_, _ = s.recomputeAndPublish(ctx, userID)
	}
	return nil
}

// Heartbeat renews a socket's liveness while it is open. It never changes status.
func (s *Service) Heartbeat(ctx context.Context, userID, clientID string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	return s.store.Heartbeat(ctx, userID, clientID)
}

// Disconnected drops a closed socket and reports whether it was the agent's LAST one
// (so the caller can debounce the auto-offline transition via Vanished). It does not
// itself flip the status — the grace window is owned by the WS handler.
func (s *Service) Disconnected(ctx context.Context, userID, clientID string) (lastGone bool, err error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return false, err
	}
	return s.store.Disconnect(ctx, userID, clientID)
}

// Vanished recomputes and publishes the agent's effective status after its last
// socket is gone — the debounced graceful path and the Redis-TTL expiry fallback both
// call it. With no live socket: an availability=online agent goes offline only when
// auto_offline is on (otherwise it STAYS online); a manual away/offline is unchanged.
// Idempotent.
func (s *Service) Vanished(ctx context.Context, userID string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := s.recomputeAndPublish(ctx, userID)
	return err
}

// List returns presence for every agent with cached presence state in the tenant,
// each carrying the EFFECTIVE status plus the raw availability + auto_offline. Load
// is recomputed in ONE aggregation (no count-per-agent N+1).
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

// ── helpers ──────────────────────────────────────────────────────────────────

// authorizeTarget resolves the target user (default self) and enforces that only the
// agent themselves — or a user.manage holder — may change it.
func (s *Service) authorizeTarget(ctx context.Context, targetUserID string) (string, error) {
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return "", apperror.Unauthorized("authentication required")
	}
	target := targetUserID
	if target == "" {
		target = ac.UserID
	}
	if target != ac.UserID && !ac.Has(authz.UserManage) {
		return "", apperror.Forbidden("cannot change another agent's presence")
	}
	return target, nil
}

// recomputeAndPublish reads the agent's durable settings + live-socket state, computes
// the effective status, caches it (so routing/List read it) and publishes the change.
func (s *Service) recomputeAndPublish(ctx context.Context, userID string) (*entity.AgentPresence, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	hasLive, err := s.store.HasLiveSocket(ctx, userID)
	if err != nil {
		return nil, err
	}
	load, err := s.load.CountOpenAssigned(ctx, userID)
	if err != nil {
		return nil, err
	}
	tenantID, _ := shared.TenantFrom(ctx)
	availability := user.AvailabilityOr()
	autoOffline := user.AutoOfflineOr()
	p := &entity.AgentPresence{
		TenantID:           tenantID,
		UserID:             userID,
		Status:             entity.ResolveEffective(availability, autoOffline, hasLive),
		Availability:       availability,
		AutoOffline:        autoOffline,
		CurrentLoad:        load,
		MaxConcurrentChats: user.MaxConcurrentChats,
		LastSeenAt:         s.clock.Now(),
	}
	if err := s.store.Save(ctx, p); err != nil {
		return nil, err
	}
	event := contracts.NewPresenceChanged(p)
	_ = s.publisher.Publish(ctx, shared.TopicPresence(tenantID), contracts.EventPresenceChanged, event)
	_ = s.publisher.Publish(ctx, shared.TopicUser(tenantID, userID), contracts.EventPresenceChanged, event)
	return p, nil
}

// validAvailability reports whether a is a settable manual availability.
func validAvailability(a string) bool {
	switch entity.Availability(a) {
	case entity.AvailabilityOnline, entity.AvailabilityAway, entity.AvailabilityOffline:
		return true
	}
	return false
}
