package shared

import (
	"context"
	"time"
)

// Locker provides best-effort distributed mutual exclusion (e.g. via Redis). It
// is used to serialize operations that must not run concurrently across nodes,
// such as conversation assignment.
type Locker interface {
	// Acquire attempts to take the lock for key with the given TTL. When
	// acquired is true the caller owns the lock and must call release when done;
	// when false the lock is held elsewhere and release is a no-op.
	Acquire(ctx context.Context, key string, ttl time.Duration) (release func(), acquired bool, err error)
}

// NoopLocker always acquires. Useful as a default and in single-node tests.
type NoopLocker struct{}

// Acquire implements Locker.
func (NoopLocker) Acquire(context.Context, string, time.Duration) (func(), bool, error) {
	return func() {}, true, nil
}
