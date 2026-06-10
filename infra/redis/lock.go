package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// unlockScript releases a lock only if the caller still owns it (token match),
// avoiding releasing a lock that has already expired and been re-acquired.
var unlockScript = goredis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end`)

// Locker is a Redis-backed implementation of shared.Locker using SET NX PX.
type Locker struct {
	rdb Client
}

// NewLocker builds the locker.
func NewLocker(rdb Client) *Locker { return &Locker{rdb: rdb} }

// Acquire takes the lock with SET key token NX PX ttl.
func (l *Locker) Acquire(ctx context.Context, key string, ttl time.Duration) (func(), bool, error) {
	token := shared.NewID()
	ok, err := l.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return func() {}, false, err
	}
	if !ok {
		return func() {}, false, nil
	}
	release := func() {
		// Best-effort release with a fresh, short-lived context.
		rctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = unlockScript.Run(rctx, l.rdb, []string{key}, token).Err()
	}
	return release, true, nil
}

var _ shared.Locker = (*Locker)(nil)
