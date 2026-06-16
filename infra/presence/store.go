// Package presence is the Redis-backed implementation of the presence store.
// Presence is operational state: a per-agent hash plus a per-tenant set of known
// agents for enumeration.
package presence

import (
	"context"
	"strconv"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/presence/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/presence/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/redis"
)

// Store implements repository.PresenceStore over Redis.
type Store struct {
	rdb redis.Client
}

// presenceTTL is the liveness window of a presence record. It is renewed on every
// WS heartbeat (every 20s); chosen equal to the socket's own pongWait (60s) so a
// presence key expires at the same moment the server would tear down a dead
// socket — tolerating ~2 missed heartbeats without a false offline on a brief
// network blip.
const presenceTTL = 60 * time.Second

// NewStore builds the store.
func NewStore(rdb redis.Client) *Store {
	return &Store{rdb: rdb}
}

// key helpers — all tenant-prefixed.
func presenceKey(tenant, user string) string { return "presence:" + tenant + ":" + user }
func agentsKey(tenant string) string         { return "presence:agents:" + tenant }

// Save upserts the presence hash, registers the agent in the tenant set and arms
// the liveness TTL. The TTL is what makes presence self-expire: the WS heartbeat
// renews it (Touch) while the socket is alive, and it lapses to offline when the
// last socket stops renewing.
func (s *Store) Save(ctx context.Context, p *entity.AgentPresence) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	key := presenceKey(tenantID, p.UserID)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{
		"status":               string(p.Status),
		"current_load":         p.CurrentLoad,
		"max_concurrent_chats": p.MaxConcurrentChats,
		"last_seen_at":         p.LastSeenAt.UnixMilli(),
	})
	pipe.SAdd(ctx, agentsKey(tenantID), p.UserID)
	pipe.Expire(ctx, key, presenceTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return apperror.Internal("presence store error").Wrap(err)
	}
	return nil
}

// Touch renews the liveness TTL of an EXISTING presence record without touching
// its stored status. Expire on a missing key is a no-op (returns false), which is
// exactly the desired semantics: a heartbeat keeps an already-declared status
// alive, but never resurrects a record — so connecting a socket does not promote
// anyone to online, and a vanished agent stays gone.
func (s *Store) Touch(ctx context.Context, userID string) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	if err := s.rdb.Expire(ctx, presenceKey(tenantID, userID), presenceTTL).Err(); err != nil {
		return apperror.Internal("presence store error").Wrap(err)
	}
	return nil
}

// Remove deletes the presence record and drops the agent from the tenant roster
// set. Called when an agent vanishes (graceful disconnect or TTL expiry) so the
// roster never accumulates dead ids. Idempotent.
func (s *Store) Remove(ctx context.Context, userID string) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, presenceKey(tenantID, userID))
	pipe.SRem(ctx, agentsKey(tenantID), userID)
	if _, err := pipe.Exec(ctx); err != nil {
		return apperror.Internal("presence store error").Wrap(err)
	}
	return nil
}

// Get returns the stored presence or a not_found AppError.
func (s *Store) Get(ctx context.Context, userID string) (*entity.AgentPresence, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	fields, err := s.rdb.HGetAll(ctx, presenceKey(tenantID, userID)).Result()
	if err != nil {
		return nil, apperror.Internal("presence store error").Wrap(err)
	}
	if len(fields) == 0 {
		return nil, apperror.NotFound("presence not found")
	}
	return fromFields(tenantID, userID, fields), nil
}

// List returns presence for every known agent in the tenant.
func (s *Store) List(ctx context.Context) ([]*entity.AgentPresence, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	userIDs, err := s.rdb.SMembers(ctx, agentsKey(tenantID)).Result()
	if err != nil {
		return nil, apperror.Internal("presence store error").Wrap(err)
	}
	out := make([]*entity.AgentPresence, 0, len(userIDs))
	for _, uid := range userIDs {
		fields, err := s.rdb.HGetAll(ctx, presenceKey(tenantID, uid)).Result()
		if err != nil || len(fields) == 0 {
			continue
		}
		out = append(out, fromFields(tenantID, uid, fields))
	}
	return out, nil
}

func fromFields(tenantID, userID string, f map[string]string) *entity.AgentPresence {
	lastSeen := time.Time{}
	if ms, err := strconv.ParseInt(f["last_seen_at"], 10, 64); err == nil {
		lastSeen = time.UnixMilli(ms).UTC()
	}
	load, _ := strconv.Atoi(f["current_load"])
	maxChats, _ := strconv.Atoi(f["max_concurrent_chats"])
	status := entity.Status(f["status"])
	if status == "" {
		status = entity.StatusOffline
	}
	return &entity.AgentPresence{
		TenantID:           tenantID,
		UserID:             userID,
		Status:             status,
		CurrentLoad:        load,
		MaxConcurrentChats: maxChats,
		LastSeenAt:         lastSeen,
	}
}

var _ repository.PresenceStore = (*Store)(nil)
