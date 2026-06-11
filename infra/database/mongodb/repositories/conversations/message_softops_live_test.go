//go:build e2e

package conversations

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	auditrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/audit/repository"
	auditservice "github.com/romerito007/chat-smsnet-omnichannel/domain/audit/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convservice "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	auditmongo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/audit"
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
	audit := auditservice.NewService(auditmongo.New(db), shared.SystemClock{})
	svc := convservice.New(
		NewConversationRepository(db),
		messages,
		NewEventRepository(db),
		sectorsrepo.New(db),
		shared.NoopPublisher{},
		shared.SystemClock{},
	)
	svc.SetAuditor(audit)

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

	// Deleting a customer message must surface in the audit trail (the same
	// audit.List that backs GET /v1/audit), tagged sender_type=customer so content
	// moderation is distinguishable.
	if _, err := db.Collection("messages").InsertOne(context.Background(), bson.M{
		"_id": "cust1", "tenant_id": tenant, "conversation_id": conv.ID,
		"sender_type": string(conventity.SenderCustomer), "direction": string(conventity.DirectionInbound),
		"message_type": string(conventity.MessageText), "text": "spam", "created_at": time.Now(),
	}); err != nil {
		t.Fatalf("seed customer message: %v", err)
	}
	if err := svc.DeleteMessage(ctx, conv.ID, "cust1"); err != nil {
		t.Fatalf("delete customer message: %v", err)
	}

	entries, err := audit.List(ctx, auditrepo.Filter{Action: "message.deleted"}, shared.PageRequest{Limit: 50})
	if err != nil {
		t.Fatalf("audit list: %v", err)
	}
	var foundCustomer bool
	for _, e := range entries {
		if e.ResourceType == "message" && e.ResourceID == "cust1" && e.Data["sender_type"] == string(conventity.SenderCustomer) {
			foundCustomer = true
			if e.ActorID != "u1" || e.ActorType != shared.ActorTypeUser {
				t.Errorf("audit actor not captured: %+v", e)
			}
		}
	}
	if !foundCustomer {
		t.Fatalf("customer-message deletion not found in GET /v1/audit data: %+v", entries)
	}
	t.Logf("audit has message.deleted for customer message (entries=%d)", len(entries))
}
