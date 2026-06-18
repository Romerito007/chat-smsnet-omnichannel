//go:build e2e

package audit

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/audit/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/audit/repository"
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
	return cl.Database("audit_e2e")
}

// TestListFiltersByActorID guards the bug where ?actor_id= was ignored: List must
// return only the given actor's logs, AND with action when both are set, and not
// filter by actor when ActorID is empty.
func TestListFiltersByActorID(t *testing.T) {
	db := connect(t)
	_ = db.Drop(context.Background())
	const tenant = "t-audit"
	ctx := shared.WithTenant(context.Background(), tenant)
	repo := New(db)
	now := time.Now().UTC()

	seed := []*entity.AuditLog{
		{ID: "l1", TenantID: tenant, ActorID: "alice", Action: "user.created", CreatedAt: now.Add(-3 * time.Minute)},
		{ID: "l2", TenantID: tenant, ActorID: "alice", Action: "channel.updated", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "l3", TenantID: tenant, ActorID: "bob", Action: "user.created", CreatedAt: now.Add(-1 * time.Minute)},
	}
	for _, l := range seed {
		if err := repo.Create(ctx, l); err != nil {
			t.Fatalf("create %s: %v", l.ID, err)
		}
	}

	page := shared.PageRequest{Limit: 50}

	// Only alice's logs.
	byActor, err := repo.List(ctx, repository.Filter{ActorID: "alice"}, page)
	if err != nil {
		t.Fatalf("list by actor: %v", err)
	}
	if len(byActor) != 2 {
		t.Fatalf("actor_id=alice must return 2 logs, got %d", len(byActor))
	}
	for _, l := range byActor {
		if l.ActorID != "alice" {
			t.Errorf("got a log from another actor: %+v", l)
		}
	}

	// actor_id + action are ANDed.
	both, err := repo.List(ctx, repository.Filter{ActorID: "alice", Action: "user."}, page)
	if err != nil {
		t.Fatalf("list by actor+action: %v", err)
	}
	if len(both) != 1 || both[0].ID != "l1" {
		t.Fatalf("actor_id=alice + action=user. must return only l1, got %+v", both)
	}

	// Empty actor_id does not filter by actor.
	all, err := repo.List(ctx, repository.Filter{}, page)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("no actor filter must return all 3, got %d", len(all))
	}
}
