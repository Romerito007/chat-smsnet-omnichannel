//go:build e2e

package channels

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/secrets"
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
	return cl.Database("channels_e2e")
}

// TestUpdatePersistsBusinessHours guards the regression where Update's $set omitted
// business_hours: a PATCH returned 200 (entity mutated in memory) but the new
// schedule was never written, so the next read served the stale one.
func TestUpdatePersistsBusinessHours(t *testing.T) {
	db := connect(t)
	_ = db.Drop(context.Background())
	cipher, err := secrets.NewCipher("a-32-byte-or-longer-test-encryption-key")
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	repo := NewConnectionRepository(db, cipher)
	ctx := shared.WithTenant(context.Background(), "t-ch")
	now := time.Now().UTC()

	conn := &entity.ChannelConnection{
		ID: shared.NewID(), TenantID: "t-ch", Type: entity.TypeAPI, Name: "WA",
		Secret: "s", InboundTokenHash: "h",
		BusinessHours: map[string]any{"timezone": "UTC"}, // initial
		Enabled:       true, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(ctx, conn); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Edit the schedule (timezone + a weekly window).
	conn.BusinessHours = map[string]any{
		"timezone": "America/Sao_Paulo",
		"weekly": []any{
			map[string]any{"day": int32(1), "intervals": []any{
				map[string]any{"start": "09:00", "end": "12:00"},
			}},
		},
	}
	conn.UpdatedAt = now.Add(time.Minute)
	if err := repo.Update(ctx, conn); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.FindByID(ctx, conn.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.BusinessHours == nil || got.BusinessHours["timezone"] != "America/Sao_Paulo" {
		t.Fatalf("business_hours not persisted on update: %+v", got.BusinessHours)
	}
	if _, ok := got.BusinessHours["weekly"]; !ok {
		t.Errorf("weekly schedule not persisted: %+v", got.BusinessHours)
	}
}
