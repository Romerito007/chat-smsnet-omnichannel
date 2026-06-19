// Package presence is the Redis-backed implementation of the presence store.
// Presence is operational state: a per-agent hash plus a per-tenant set of known
// agents for enumeration.
package presence

import (
	"context"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

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
func connsKey(tenant, user string) string    { return "presence:conns:" + tenant + ":" + user }

// Save upserts the effective-status hash and registers the agent in the tenant set.
// The hash is the CACHE of the computed effective status (read by Get/List and by
// routing); it carries no TTL — transitions are driven by explicit events and by the
// liveness of the connection set (the conns ZSET), not by this hash expiring.
func (s *Store) Save(ctx context.Context, p *entity.AgentPresence) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	key := presenceKey(tenantID, p.UserID)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{
		"status":               string(p.Status),
		"availability":         p.Availability,
		"auto_offline":         boolToStr(p.AutoOffline),
		"current_load":         p.CurrentLoad,
		"max_concurrent_chats": p.MaxConcurrentChats,
		"last_seen_at":         p.LastSeenAt.UnixMilli(),
	})
	pipe.SAdd(ctx, agentsKey(tenantID), p.UserID)
	if _, err := pipe.Exec(ctx); err != nil {
		return apperror.Internal("presence store error").Wrap(err)
	}
	return nil
}

// Connect records a live socket (clientID) for the user in the per-user connection
// ZSET (clientID → last-seen millis) and arms the liveness TTL on that key. It
// reports whether this is the FIRST live socket (the user went from no-socket to
// has-socket), so the service can flip the effective status to online.
func (s *Store) Connect(ctx context.Context, userID, clientID string) (becameLive bool, err error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return false, err
	}
	key := connsKey(tenantID, userID)
	s.prune(ctx, key)
	before, _ := s.rdb.ZCard(ctx, key).Result()
	pipe := s.rdb.TxPipeline()
	pipe.ZAdd(ctx, key, goredis.Z{Score: float64(s.now()), Member: clientID})
	pipe.Expire(ctx, key, presenceTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, apperror.Internal("presence store error").Wrap(err)
	}
	return before == 0, nil
}

// Heartbeat renews a socket's liveness (ZADD now + renew TTL). Never changes status.
func (s *Store) Heartbeat(ctx context.Context, userID, clientID string) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	key := connsKey(tenantID, userID)
	pipe := s.rdb.TxPipeline()
	pipe.ZAdd(ctx, key, goredis.Z{Score: float64(s.now()), Member: clientID})
	pipe.Expire(ctx, key, presenceTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return apperror.Internal("presence store error").Wrap(err)
	}
	return nil
}

// Disconnect drops a socket from the connection set and reports whether the user now
// has NO live socket (the LAST one closed), so the service can apply the auto-offline
// rule. Idempotent.
func (s *Store) Disconnect(ctx context.Context, userID, clientID string) (lastGone bool, err error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return false, err
	}
	key := connsKey(tenantID, userID)
	if err := s.rdb.ZRem(ctx, key, clientID).Err(); err != nil {
		return false, apperror.Internal("presence store error").Wrap(err)
	}
	s.prune(ctx, key)
	n, _ := s.rdb.ZCard(ctx, key).Result()
	if n == 0 {
		_ = s.rdb.Del(ctx, key).Err()
		return true, nil
	}
	return false, nil
}

// HasLiveSocket reports whether the user currently has any non-stale live socket.
func (s *Store) HasLiveSocket(ctx context.Context, userID string) (bool, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return false, err
	}
	key := connsKey(tenantID, userID)
	s.prune(ctx, key)
	n, err := s.rdb.ZCard(ctx, key).Result()
	if err != nil {
		return false, apperror.Internal("presence store error").Wrap(err)
	}
	return n > 0, nil
}

// now is the current epoch in milliseconds (the ZSET score unit).
func (s *Store) now() int64 { return time.Now().UnixMilli() }

// prune drops connection entries whose last-seen is older than the liveness TTL
// (self-healing for sockets that died without a graceful Disconnect).
func (s *Store) prune(ctx context.Context, key string) {
	cutoff := time.Now().Add(-presenceTTL).UnixMilli()
	_ = s.rdb.ZRemRangeByScore(ctx, key, "-inf", "("+strconv.FormatInt(cutoff, 10)).Err()
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
	pipe.Del(ctx, connsKey(tenantID, userID))
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
		Availability:       f["availability"],
		AutoOffline:        f["auto_offline"] == "1",
		CurrentLoad:        load,
		MaxConcurrentChats: maxChats,
		LastSeenAt:         lastSeen,
	}
}

func boolToStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

var _ repository.PresenceStore = (*Store)(nil)
