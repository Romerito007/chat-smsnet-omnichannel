//go:build e2e

package attachments

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	acontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/contracts"
	aentity "github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/entity"
	aservice "github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/service"
	areposcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/audit/repository"
	auditservice "github.com/romerito007/chat-smsnet-omnichannel/domain/audit/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	auditrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/audit"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/storage"
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
	return cl.Database("mvp_e2e")
}

func adminCtx(tenant, user string) context.Context {
	ctx := shared.WithTenant(context.Background(), tenant)
	ctx = shared.WithAuditMeta(ctx, "203.0.113.7", "Go-e2e/1.0")
	return authz.WithAuthContext(ctx, authz.NewAuthContext(tenant, user, authz.AllPermissions(), nil, authz.ScopeAll))
}

func TestAttachmentsAndAuditLive(t *testing.T) {
	db := connect(t)
	_ = db.Drop(context.Background())
	const tenant = "t-mvp"
	ctx := adminCtx(tenant, "owner-1")

	// Seed a conversation + a message to link the attachment to.
	if _, err := db.Collection("conversations").InsertOne(context.Background(), bson.M{
		"_id": "cv1", "tenant_id": tenant, "contact_id": "c1", "channel": "whatsapp",
		"status": "assigned", "sector_id": "s1", "created_at": time.Now(),
	}); err != nil {
		t.Fatalf("seed conv: %v", err)
	}
	if _, err := db.Collection("messages").InsertOne(context.Background(), bson.M{
		"_id": "m1", "tenant_id": tenant, "conversation_id": "cv1", "sender_type": "agent",
		"direction": "outbound", "message_type": "text", "text": "see attachment", "created_at": time.Now(),
	}); err != nil {
		t.Fatalf("seed msg: %v", err)
	}

	store := storage.NewLocalAttachmentStorage(t.TempDir(), "e2e-secret", "http://localhost:8080")
	svc := aservice.NewService(
		New(db),
		store,
		convrepo.NewConversationRepository(db),
		convrepo.NewMessageRepository(db),
		nil,
		aservice.Config{MaxSizeBytes: 1 << 20, AllowedContentTypes: []string{"image/*"}, DownloadBaseURL: "http://localhost:8080"},
	)

	// 1. upload-url
	att, target, err := svc.RequestUploadURL(ctx, acontracts.CreateUploadURL{
		ConversationID: "cv1", Filename: "pic.png", ContentType: "image/png", Size: 5,
	})
	if err != nil {
		t.Fatalf("upload-url: %v", err)
	}
	if att.StorageProvider != "local" || target.Method != "PUT" {
		t.Fatalf("unexpected: %+v %+v", att, target)
	}

	// 2. direct upload to storage (simulating the blob PUT endpoint)
	token := target.URL[len("http://localhost:8080/v1/attachments/blobs/"):]
	key, ct, _, err := store.ResolveUpload(token)
	if err != nil {
		t.Fatalf("resolve upload token: %v", err)
	}
	if err := store.Put(key, ct, []byte("BYTES")); err != nil {
		t.Fatalf("put: %v", err)
	}

	// 3. confirm + link to message
	confirmed, err := svc.Confirm(ctx, acontracts.ConfirmUpload{AttachmentID: att.ID, MessageID: "m1"})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if confirmed.Status != "ready" || confirmed.MessageID != "m1" {
		t.Fatalf("not linked/ready: %+v", confirmed)
	}

	// Persisted + linked in Mongo.
	var doc bson.M
	if err := db.Collection("attachments").FindOne(context.Background(), bson.M{"_id": att.ID}).Decode(&doc); err != nil {
		t.Fatalf("find attachment: %v", err)
	}
	if doc["message_id"] != "m1" || doc["status"] != "ready" {
		t.Errorf("attachment not linked in mongo: %v", doc)
	}

	// 4. download (access-checked) returns the bytes
	res, err := svc.Download(ctx, att.ID)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if string(res.Data) != "BYTES" || res.ContentType != "image/png" {
		t.Errorf("unexpected download: %+v", res)
	}
	t.Logf("attachment %s linked to message m1, %d bytes, download url %s", att.ID, len(res.Data), confirmed.SignedURL)

	// 5. AUDIT: actor_type / ip / user_agent captured + filterable by action.
	audit := auditservice.NewService(auditrepo.New(db), nil)
	if err := audit.Record(ctx, shared.AuditEntry{Action: "user.created", ResourceType: "user", ResourceID: "u2"}); err != nil {
		t.Fatalf("audit record: %v", err)
	}
	var alog bson.M
	if err := db.Collection("audit_logs").FindOne(context.Background(), bson.M{"action": "user.created"}).Decode(&alog); err != nil {
		t.Fatalf("find audit: %v", err)
	}
	if alog["actor_id"] != "owner-1" || alog["actor_type"] != "user" {
		t.Errorf("actor not captured: %v", alog)
	}
	if alog["ip"] != "203.0.113.7" || alog["user_agent"] != "Go-e2e/1.0" {
		t.Errorf("ip/user_agent not captured: %v", alog)
	}
	items, err := audit.List(ctx, areposcontracts.Filter{Action: "user."}, shared.PageRequest{Limit: 10})
	if err != nil || len(items) != 1 {
		t.Fatalf("audit list by action prefix: n=%d err=%v", len(items), err)
	}
	t.Logf("audit ok: actor=%s type=%s ip=%s ua=%s", alog["actor_id"], alog["actor_type"], alog["ip"], alog["user_agent"])
}

// TestAttachmentUpdatePersistsRemuxedFields guards the regression where Update's $set
// omitted content_type/filename/storage_key/size: the audio remux (webm->ogg) on
// Confirm mutated those, but only signed_url reached Mongo, so IntegrationMediaURL
// re-read the stale webm content_type and served an extension-less URL to the gateway.
func TestAttachmentUpdatePersistsRemuxedFields(t *testing.T) {
	db := connect(t)
	_ = db.Drop(context.Background())
	const tenant = "t-remux"
	ctx := adminCtx(tenant, "owner-1")
	repo := New(db)

	att := &aentity.Attachment{
		ID: "a1", TenantID: tenant, ConversationID: "cv1",
		Filename: "voice.webm", ContentType: "audio/webm;codecs=opus",
		StorageKey: "attachments/t/cv1/a1/voice.webm", Size: 10,
		StorageProvider: "local", Status: aentity.StatusPending, CreatedAt: time.Now(),
	}
	if err := repo.Create(ctx, att); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Simulate the remux mutating the record in place, then persist.
	att.ContentType = "audio/ogg"
	att.Filename = "voice.ogg"
	att.StorageKey = "attachments/t/cv1/a1/voice.ogg"
	att.Size = 7
	att.Status = aentity.StatusReady
	att.SignedURL = "http://localhost:8080/v1/channel-media/tok.ogg"
	if err := repo.Update(ctx, att); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.FindByID(ctx, "a1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.ContentType != "audio/ogg" {
		t.Errorf("content_type not persisted: %q", got.ContentType)
	}
	if got.Filename != "voice.ogg" {
		t.Errorf("filename not persisted: %q", got.Filename)
	}
	if got.StorageKey != "attachments/t/cv1/a1/voice.ogg" {
		t.Errorf("storage_key not persisted: %q", got.StorageKey)
	}
	if got.Size != 7 {
		t.Errorf("size not persisted: %d", got.Size)
	}
}
