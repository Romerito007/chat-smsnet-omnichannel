//go:build e2e

package reports

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/reports/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

func connect(t *testing.T) *mongo.Database {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cl, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := cl.Ping(ctx, nil); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return cl.Database("reports_e2e")
}

// TestFirstResponseAvg_FromEvents verifies the SLA-independent first-response metric:
// it averages (first agent message.created event − conversation created_at) across
// conversations created in the period, excluding conversations with no agent reply.
func TestFirstResponseAvg_FromEvents(t *testing.T) {
	db := connect(t)
	ctx := shared.WithTenant(context.Background(), "t-e2e")
	_ = db.Collection("conversations").Drop(ctx)
	_ = db.Collection("conversation_events").Drop(ctx)

	mk := func(s string) time.Time { tm, _ := time.Parse(time.RFC3339Nano, s); return tm }
	// A: created 16:30:35.070, first AGENT reply 16:32:13.610 → 98.54s
	_, _ = db.Collection("conversations").InsertOne(ctx, map[string]any{
		"_id": "A", "tenant_id": "t-e2e", "created_at": mk("2026-06-15T16:30:35.070Z"), "channel": "api",
	})
	// B: created 18:17:16, first agent reply 18:18:16 → 60s
	_, _ = db.Collection("conversations").InsertOne(ctx, map[string]any{
		"_id": "B", "tenant_id": "t-e2e", "created_at": mk("2026-06-15T18:17:16.000Z"), "channel": "api",
	})
	// C: created 20:00, only an AUTOMATION reply → excluded from the average.
	_, _ = db.Collection("conversations").InsertOne(ctx, map[string]any{
		"_id": "C", "tenant_id": "t-e2e", "created_at": mk("2026-06-15T20:00:00.000Z"), "channel": "api",
	})
	ev := func(cid, actor string, at string) map[string]any {
		return map[string]any{"conversation_id": cid, "tenant_id": "t-e2e", "type": "message.created", "actor_type": actor, "created_at": mk(at)}
	}
	_, _ = db.Collection("conversation_events").InsertMany(ctx, []any{
		ev("A", "system", "2026-06-15T16:30:35.609Z"), // inbound — ignored
		ev("A", "agent", "2026-06-15T16:32:13.610Z"),  // first agent reply
		ev("A", "agent", "2026-06-15T16:40:58.000Z"),  // later — ignored
		ev("B", "agent", "2026-06-15T18:18:16.000Z"),
		ev("C", "automation", "2026-06-15T20:00:05.000Z"), // not an agent — C excluded
	})

	r := New(db)
	f := contracts.Filter{From: mk("2026-06-09T00:00:00Z"), To: mk("2026-06-16T23:59:59Z")}
	avg, err := r.FirstResponseAvgSeconds(ctx, f)
	if err != nil {
		t.Fatalf("first response: %v", err)
	}
	// (98.54 + 60) / 2 = 79.27 (C excluded; later A reply ignored).
	if avg < 79.0 || avg > 79.5 {
		t.Fatalf("first_response_avg_seconds = %v, want ~79.27", avg)
	}
}
