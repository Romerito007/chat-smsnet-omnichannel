//go:build e2e

package privacy

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	auditservice "github.com/romerito007/chat-smsnet-omnichannel/domain/audit/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	pcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/contracts"
	privservice "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/service"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	auditrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/audit"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/storage"
)

type captureEnqueuer struct{ task *pcontracts.ExportTask }

func (e *captureEnqueuer) EnqueueExport(t pcontracts.ExportTask) error { e.task = &t; return nil }

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
	return cl.Database("privacy_e2e")
}

func adminCtx(tenantID, userID string) context.Context {
	ctx := shared.WithTenant(context.Background(), tenantID)
	return authz.WithAuthContext(ctx, authz.NewAuthContext(tenantID, userID, authz.AllPermissions(), nil, authz.ScopeAll))
}

// TestPrivacyEraseContact_CascadeUnlinkAndBlob covers the right-to-be-forgotten
// erasure: a contact with a linked deal is blocked (409) until force=true, then
// the contact + every satellite document and its media blob are hard-deleted
// while the deal is kept but unlinked. Requires a live Mongo (-tags e2e).
func TestPrivacyEraseContact_CascadeUnlinkAndBlob(t *testing.T) {
	db := connect(t)
	_ = db.Drop(context.Background())
	bg := context.Background()
	const tenant = "t-erase"
	ctx := adminCtx(tenant, "owner-1")
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	mustInsert(t, db, "contacts", bson.M{
		"_id": "ce", "tenant_id": tenant, "name": "Erase Me", "phone": "11999990000",
		"document": "111.444.777-35", "email": "erase@x.com", "created_at": now,
	})
	mustInsert(t, db, "conversations", bson.M{
		"_id": "cve", "tenant_id": tenant, "contact_id": "ce", "channel": "whatsapp",
		"status": "open", "created_at": now,
	})
	mustInsert(t, db, "messages", bson.M{
		"_id": "me1", "tenant_id": tenant, "conversation_id": "cve", "sender_type": "customer",
		"direction": "inbound", "message_type": "text", "text": "oi", "created_at": now,
	})
	score := 5
	mustInsert(t, db, "csat_responses", bson.M{
		"_id": "cse", "tenant_id": tenant, "conversation_id": "cve", "contact_id": "ce",
		"survey_id": "s1", "token": "tk", "score": &score, "status": "responded", "created_at": now,
	})
	// A deal linked to the contact (directly + via conversation) — kept, not deleted.
	mustInsert(t, db, "deals", bson.M{
		"_id": "de1", "tenant_id": tenant, "contact_id": "ce", "conversation_ids": bson.A{"cve"},
		"pipeline_id": "p1", "stage_id": "s1", "title": "Won deal", "status": "open",
		"stage_changed_at": now, "created_at": now,
	})

	blobs := storage.NewLocalAttachmentStorage(t.TempDir(), "s", "http://x")
	const blobKey = "blobs/ce/att.bin"
	if err := blobs.Put(blobKey, "application/octet-stream", []byte("media")); err != nil {
		t.Fatalf("seed blob: %v", err)
	}
	mustInsert(t, db, "attachments", bson.M{
		"_id": "at1", "tenant_id": tenant, "conversation_id": "cve", "message_id": "me1",
		"filename": "att.bin", "content_type": "application/octet-stream", "size": int64(5),
		"storage_provider": "local", "storage_key": blobKey, "status": "ready", "created_at": now,
	})

	store := New(db)
	auditor := auditservice.NewService(auditrepo.New(db), fixedClock{now})
	files := storage.NewLocalFileStore(t.TempDir(), "s", "http://x")
	svc := privservice.NewService(store, files, blobs, &captureEnqueuer{}, auditor, fixedClock{now}, time.Hour)

	// 1) Without force → blocked (conflict), nothing deleted.
	if err := svc.EraseContact(ctx, "ce", false); err == nil {
		t.Fatalf("erase must be blocked while a deal is linked")
	}
	if count(t, db, "contacts", bson.M{"_id": "ce"}) != 1 {
		t.Fatalf("blocked erase must delete nothing")
	}

	// 2) With force → cascade erase + deal unlink + blob purge.
	if err := svc.EraseContact(ctx, "ce", true); err != nil {
		t.Fatalf("forced erase: %v", err)
	}
	for _, c := range []struct {
		coll   string
		filter bson.M
	}{
		{"contacts", bson.M{"_id": "ce"}},
		{"conversations", bson.M{"_id": "cve"}},
		{"messages", bson.M{"_id": "me1"}},
		{"csat_responses", bson.M{"_id": "cse"}},
		{"attachments", bson.M{"_id": "at1"}},
	} {
		if count(t, db, c.coll, c.filter) != 0 {
			t.Errorf("%s row must be erased", c.coll)
		}
	}
	if ok, _ := blobs.Exists(blobKey); ok {
		t.Errorf("attachment media blob must be purged")
	}
	// The deal survives, unlinked from the erased contact.
	var deal bson.M
	if err := db.Collection("deals").FindOne(bg, bson.M{"_id": "de1"}).Decode(&deal); err != nil {
		t.Fatalf("deal must be kept: %v", err)
	}
	if deal["contact_id"] != "" {
		t.Errorf("deal contact_id must be cleared, got %v", deal["contact_id"])
	}
	if convs, ok := deal["conversation_ids"].(bson.A); ok && len(convs) != 0 {
		t.Errorf("deal conversation_ids must be pulled, got %v", convs)
	}
	if deal["title"] != "Won deal" {
		t.Errorf("deal title must be preserved, got %v", deal["title"])
	}
	if count(t, db, "audit_logs", bson.M{"tenant_id": tenant, "action": "privacy.contact.erased"}) == 0 {
		t.Errorf("erasure must be audited")
	}
	if count(t, db, "audit_logs", bson.M{"tenant_id": tenant, "action": "privacy.contact.erase_blocked"}) == 0 {
		t.Errorf("the blocked attempt must be audited")
	}
}

func TestPrivacyLiveEndToEnd(t *testing.T) {
	db := connect(t)
	_ = db.Drop(context.Background())
	bg := context.Background()
	const tenant = "t-live"
	ctx := adminCtx(tenant, "owner-1")

	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	clock := fixedClock{now}

	// --- seed data: two contacts, conversations, messages, csat, plus other collections ---
	mustInsert(t, db, "contacts", bson.M{
		"_id": "c1", "tenant_id": tenant, "name": "João Silva", "phone": "11999998888",
		"document":   "529.982.247-25",
		"identities": bson.A{bson.M{"channel": "whatsapp", "external_id": "5511999998888"}},
		"created_at": now.Add(-100 * 24 * time.Hour),
	})
	mustInsert(t, db, "contacts", bson.M{
		"_id": "c2", "tenant_id": tenant, "name": "Maria Held", "phone": "11888887777",
		"created_at": now.Add(-100 * 24 * time.Hour),
	})
	mustInsert(t, db, "conversations", bson.M{
		"_id": "cv1", "tenant_id": tenant, "contact_id": "c1", "channel": "whatsapp",
		"status": "closed", "created_at": now.Add(-90 * 24 * time.Hour), "closed_at": now.Add(-89 * 24 * time.Hour),
	})
	mustInsert(t, db, "conversations", bson.M{
		"_id": "cv2", "tenant_id": tenant, "contact_id": "c2", "channel": "whatsapp",
		"status": "closed", "created_at": now.Add(-90 * 24 * time.Hour), "closed_at": now.Add(-89 * 24 * time.Hour),
	})
	// old message for c1 (subject to retention), with PII in text
	mustInsert(t, db, "messages", bson.M{
		"_id": "m1", "tenant_id": tenant, "conversation_id": "cv1", "sender_type": "customer",
		"direction": "inbound", "message_type": "text",
		"text":       "Meu nome é João Silva, telefone 11999998888, email joao@x.com",
		"created_at": now.Add(-89 * 24 * time.Hour),
	})
	// recent message for c1 (not subject to 30d retention)
	mustInsert(t, db, "messages", bson.M{
		"_id": "m2", "tenant_id": tenant, "conversation_id": "cv1", "sender_type": "agent",
		"direction": "outbound", "message_type": "text", "text": "Obrigado pelo contato",
		"created_at": now.Add(-1 * 24 * time.Hour),
	})
	// old message for HELD contact c2 — must survive retention
	mustInsert(t, db, "messages", bson.M{
		"_id": "m3", "tenant_id": tenant, "conversation_id": "cv2", "sender_type": "customer",
		"direction": "inbound", "message_type": "text", "text": "mensagem antiga do contato sob hold",
		"created_at": now.Add(-89 * 24 * time.Hour),
	})
	score := 5
	mustInsert(t, db, "csat_responses", bson.M{
		"_id": "cs1", "tenant_id": tenant, "conversation_id": "cv1", "contact_id": "c1",
		"survey_id": "s1", "token": "tok", "score": &score, "status": "responded",
		"created_at": now.Add(-88 * 24 * time.Hour),
	})
	// legal hold on c2 (indefinite)
	mustInsert(t, db, "legal_holds", bson.M{
		"_id": "lh1", "tenant_id": tenant, "contact_id": "c2", "reason": "litigation", "created_at": now,
	})
	// old audit + notification (subject to their own retention)
	mustInsert(t, db, "audit_logs", bson.M{
		"_id": "a-old", "tenant_id": tenant, "action": "old.thing", "created_at": now.Add(-400 * 24 * time.Hour),
	})
	mustInsert(t, db, "notifications", bson.M{
		"_id": "n-old", "tenant_id": tenant, "created_at": now.Add(-60 * 24 * time.Hour),
	})

	store := New(db)
	files := storage.NewLocalFileStore(t.TempDir(), "e2e-secret", "http://localhost:8080")
	blobs := storage.NewLocalAttachmentStorage(t.TempDir(), "e2e-secret", "http://localhost:8080")
	auditor := auditservice.NewService(auditrepo.New(db), clock)
	enq := &captureEnqueuer{}
	svc := privservice.NewService(store, files, blobs, enq, auditor, clock, time.Hour)

	// === 1. EXPORT: request → run → file via signed URL contains chat data, no provider data ===
	req, err := svc.RequestExport(ctx, "c1")
	if err != nil {
		t.Fatalf("request export: %v", err)
	}
	if enq.task == nil || enq.task.RequestID != req.ID {
		t.Fatalf("export job not enqueued")
	}
	if err := svc.RunExport(ctx, req.ID); err != nil {
		t.Fatalf("run export: %v", err)
	}
	done, _ := svc.GetExport(ctx, req.ID)
	if done.Status != "ready" || done.DownloadURL == "" {
		t.Fatalf("export not ready: %+v", done)
	}
	// resolve the signed URL like the public download endpoint would
	u, _ := url.Parse(done.DownloadURL)
	token := u.Path[strings.LastIndex(u.Path, "/")+1:]
	key, err := files.Resolve(token)
	if err != nil {
		t.Fatalf("resolve signed url: %v", err)
	}
	data, _, err := files.Open(key)
	if err != nil {
		t.Fatalf("open export file: %v", err)
	}
	var bundle map[string]any
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("bundle json: %v", err)
	}
	if bundle["contact"] == nil || bundle["conversations"] == nil || bundle["csat"] == nil {
		t.Fatalf("bundle missing sections: %v", string(data))
	}
	if strings.Contains(string(data), "provider") {
		t.Errorf("export must not contain provider data")
	}
	t.Logf("export bundle bytes=%d", len(data))

	// === 2. LEGAL HOLD: erasing c2 is refused and deletes nothing ===
	if err := svc.EraseContact(ctx, "c2", true); err == nil {
		t.Fatalf("expected legal-hold refusal for c2")
	}
	var c2 bson.M
	_ = db.Collection("contacts").FindOne(bg, bson.M{"_id": "c2"}).Decode(&c2)
	if c2["name"] != "Maria Held" {
		t.Errorf("held contact must not be erased: %v", c2)
	}
	if count(t, db, "messages", bson.M{"_id": "m3"}) != 1 {
		t.Errorf("held contact's message must survive a refused erase")
	}

	// === 4. RETENTION: configure + apply; old data gone, held + recent preserved ===
	d30, d365, d30b := 30, 365, 30
	if _, err := svc.UpdateRetention(ctx, pcontracts.UpdateRetention{
		MessagesDays: &d30, AuditLogsDays: &d365, NotificationsDays: &d30b,
	}); err != nil {
		t.Fatalf("update retention: %v", err)
	}
	deleted, err := svc.ApplyRetention(ctx)
	if err != nil {
		t.Fatalf("apply retention: %v", err)
	}
	t.Logf("retention deleted=%d", deleted)
	if count(t, db, "messages", bson.M{"_id": "m1"}) != 0 {
		t.Errorf("old message m1 should be deleted by retention")
	}
	if count(t, db, "messages", bson.M{"_id": "m2"}) != 1 {
		t.Errorf("recent message m2 must be kept")
	}
	if count(t, db, "messages", bson.M{"_id": "m3"}) != 1 {
		t.Errorf("held contact's old message m3 must be preserved (legal hold)")
	}
	if count(t, db, "audit_logs", bson.M{"_id": "a-old"}) != 0 {
		t.Errorf("old audit log should be deleted")
	}
	if count(t, db, "notifications", bson.M{"_id": "n-old"}) != 0 {
		t.Errorf("old notification should be deleted")
	}

	// === 5. AUDIT: every action left a trail ===
	for _, action := range []string{
		"privacy.export.requested", "privacy.export.generated",
		"privacy.contact.erase_refused",
		"privacy.retention.updated", "privacy.retention.applied",
	} {
		if count(t, db, "audit_logs", bson.M{"tenant_id": tenant, "action": action}) == 0 {
			t.Errorf("missing audit entry for %q", action)
		}
	}
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func mustInsert(t *testing.T, db *mongo.Database, coll string, doc bson.M) {
	t.Helper()
	if _, err := db.Collection(coll).InsertOne(context.Background(), doc); err != nil {
		t.Fatalf("insert %s: %v", coll, err)
	}
}

func count(t *testing.T, db *mongo.Database, coll string, filter bson.M) int64 {
	t.Helper()
	n, err := db.Collection(coll).CountDocuments(context.Background(), filter)
	if err != nil {
		t.Fatalf("count %s: %v", coll, err)
	}
	return n
}
