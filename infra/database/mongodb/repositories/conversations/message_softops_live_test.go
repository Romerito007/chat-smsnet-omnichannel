//go:build e2e

package conversations

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convservice "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	sectorsrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/sectors"
)

func connectMsg(t *testing.T) *mongo.Database {
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
	return cl.Database("msg_softops_e2e")
}

func actorCtx(tenant, user string) context.Context {
	ctx := shared.WithTenant(context.Background(), tenant)
	return authz.WithAuthContext(ctx, authz.NewAuthContext(tenant, user, authz.AllPermissions(), nil, authz.ScopeAll))
}

// TestMessageSoftOpsLive exercises edit/delete against real MongoDB: edited_at and
// deleted_at must persist, the deleted message must vanish from listings but stay
// in the collection, and timeline events must be recorded.
func TestMessageSoftOpsLive(t *testing.T) {
	db := connectMsg(t)
	_ = db.Drop(context.Background())
	const tenant = "t-soft"
	ctx := actorCtx(tenant, "u1")

	messages := NewMessageRepository(db)
	svc := convservice.New(
		NewConversationRepository(db),
		messages,
		NewEventRepository(db),
		sectorsrepo.New(db),
		shared.NoopPublisher{},
		shared.SystemClock{},
	)

	conv, err := svc.Create(ctx, convcontracts.CreateConversation{ContactID: "c1", Channel: "wa"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	msg, err := svc.SendMessage(ctx, conv.ID, convcontracts.SendMessage{MessageType: conventity.MessageText, Text: "original"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	// EDIT
	if _, err := svc.EditMessage(ctx, conv.ID, msg.ID, convcontracts.EditMessage{Text: "edited"}); err != nil {
		t.Fatalf("edit: %v", err)
	}
	got, err := messages.FindByID(ctx, msg.ID)
	if err != nil {
		t.Fatalf("reload after edit: %v", err)
	}
	if got.Text != "edited" || got.EditedAt == nil {
		t.Errorf("edit not persisted: text=%q edited_at=%v", got.Text, got.EditedAt)
	}
	t.Logf("edited: text=%q edited_at=%v", got.Text, got.EditedAt)

	// DELETE
	if err := svc.DeleteMessage(ctx, conv.ID, msg.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// Gone from listings…
	list, _ := svc.ListMessages(ctx, conv.ID, shared.PageRequest{Limit: 50})
	for _, m := range list {
		if m.ID == msg.ID {
			t.Errorf("deleted message must not appear in listings")
		}
	}
	// …but still in the collection (history preserved) with deleted_at set.
	var raw bson.M
	if err := db.Collection("messages").FindOne(context.Background(), bson.M{"_id": msg.ID}).Decode(&raw); err != nil {
		t.Fatalf("deleted message must remain in the database: %v", err)
	}
	if raw["deleted_at"] == nil {
		t.Errorf("deleted_at must be persisted, got %v", raw)
	}
	t.Logf("soft-deleted: row present, deleted_at=%v", raw["deleted_at"])

	// Timeline events recorded for both operations.
	for _, typ := range []string{conventity.EventMessageEdited, conventity.EventMessageDeleted} {
		n, err := db.Collection("conversation_events").CountDocuments(context.Background(),
			bson.M{"tenant_id": tenant, "type": typ})
		if err != nil || n == 0 {
			t.Errorf("expected timeline event %q (n=%d err=%v)", typ, n, err)
		}
	}
}
