//go:build e2e

package groups

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/groups/contracts"
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
	return cl.Database("groups_e2e")
}

// TestUpsertBatchIsIdempotentAndPreservesAttend guards the core sync invariant: a
// re-sync of the same group_jid must NOT create a duplicate and must NOT reset the
// operator's attend choice; it only refreshes the metadata.
func TestUpsertBatchIsIdempotentAndPreservesAttend(t *testing.T) {
	db := connect(t)
	_ = db.Drop(context.Background())
	repo := New(db, shared.SystemClock{})
	ctx := shared.WithTenant(context.Background(), "t-grp")

	// First sync: one group, born with attend=true.
	if _, err := repo.UpsertBatch(ctx, "ch1", []contracts.UpsertGroup{
		{GroupJID: "120@g.us", Name: "Cliente A", Description: "old"},
	}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	g, err := repo.FindByJID(ctx, "120@g.us")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if !g.Attend {
		t.Fatalf("a new group must default to attend=true")
	}

	// The operator marks it NOT to attend.
	if _, err := repo.SetAttend(ctx, g.ID, false); err != nil {
		t.Fatalf("set attend: %v", err)
	}

	// Re-sync with refreshed metadata (the gateway pushes the same group again).
	if _, err := repo.UpsertBatch(ctx, "ch1", []contracts.UpsertGroup{
		{GroupJID: "120@g.us", Name: "Cliente A renomeado", Description: "new"},
	}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	// Exactly one document for the JID (no duplicate).
	n, err := db.Collection("whatsapp_groups").CountDocuments(ctx, bson.M{"tenant_id": "t-grp", "group_jid": "120@g.us"})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("want exactly 1 document for the JID, got %d", n)
	}

	g2, err := repo.FindByJID(ctx, "120@g.us")
	if err != nil {
		t.Fatalf("find after re-sync: %v", err)
	}
	if g2.Attend {
		t.Errorf("re-sync must NOT reset the operator's attend=false choice")
	}
	if g2.Name != "Cliente A renomeado" || g2.Description != "new" {
		t.Errorf("re-sync must refresh metadata, got name=%q desc=%q", g2.Name, g2.Description)
	}
	if g2.ID != g.ID {
		t.Errorf("re-sync must keep the same id, was %q now %q", g.ID, g2.ID)
	}
}
