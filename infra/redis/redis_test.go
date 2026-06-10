package redis

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/app/config"
)

// TestConnectPing is an integration test gated on REDIS_TEST_ADDR. It verifies
// Connect dials and pings a real Redis. Skipped when the env var is absent so
// `go test ./...` stays hermetic in CI without Redis.
func TestConnectPing(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("set REDIS_TEST_ADDR to run the Redis integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Connect(ctx, config.RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
