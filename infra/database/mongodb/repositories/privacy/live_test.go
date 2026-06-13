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

// TestPrivacyAnonymize_NoIdentitiesAndIdempotent reproduces the 500 "database
// error" bug: anonymizing a contact that has NO identities array used to fail on
// the all-positional `identities.$[]` update. It also covers a SECOND such
// contact (no unique-index collision on the cleared PII) and a re-anonymize
// (idempotent no-op). Requires a live Mongo (-tags e2e).
func TestPrivacyAnonymize_NoIdentitiesAndIdempotent(t *testing.T) {
	db := connect(t)
	_ = db.Drop(context.Background())
	bg := context.Background()
	const tenant = "t-anon"
	ctx := adminCtx(tenant, "owner-1")
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	// Two contacts WITHOUT any identities array (the root-cause shape), with PII.
	mustInsert(t, db, "contacts", bson.M{
		"_id": "ca", "tenant_id": tenant, "name": "Ana Lima", "phone": "11999990000",
		"phones": bson.A{"11999990000"}, "document": "111.444.777-35", "email": "ana@x.com",
		"created_at": now,
	})
	mustInsert(t, db, "contacts", bson.M{
		"_id": "cb", "tenant_id": tenant, "name": "Bruno Costa", "phone": "11888880000",
		"phones": bson.A{"11888880000"}, "document": "529.982.247-25", "email": "bruno@x.com",
		"created_at": now,
	})

	store := New(db)
	auditor := auditservice.NewService(auditrepo.New(db), fixedClock{now})
	svc := privservice.NewService(store, storage.NewLocalFileStore(t.TempDir(), "s", "http://x"), &captureEnqueuer{}, auditor, fixedClock{now}, time.Hour)

	// 1) First contact (no identities) anonymizes — no 500.
	if err := svc.Anonymize(ctx, "ca"); err != nil {
		t.Fatalf("anonymize ca (no identities) must succeed, got: %v", err)
	}
	// 2) Second contact anonymizes — both end with email/document="" but there is
	// no unique index on contacts PII, so no E11000 collision.
	if err := svc.Anonymize(ctx, "cb"); err != nil {
		t.Fatalf("anonymize a SECOND contact must succeed (no unique collision), got: %v", err)
	}
	// 3) Re-anonymize the first — idempotent, no error.
	if err := svc.Anonymize(ctx, "ca"); err != nil {
		t.Fatalf("re-anonymize must be idempotent, got: %v", err)
	}

	for _, id := range []string{"ca", "cb"} {
		var c bson.M
		_ = db.Collection("contacts").FindOne(bg, bson.M{"_id": id}).Decode(&c)
		if c["name"] != "Contato Anonimizado" || c["phone"] != "" || c["document"] != "" || c["email"] != "" {
			t.Errorf("%s PII not fully cleared: %v", id, c)
		}
		if phones, ok := c["phones"].(bson.A); !ok || len(phones) != 0 {
			t.Errorf("%s phones must be cleared, got %v", id, c["phones"])
		}
		if c["anonymized"] != true {
			t.Errorf("%s must be flagged anonymized", id)
		}
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
	auditor := auditservice.NewService(auditrepo.New(db), clock)
	enq := &captureEnqueuer{}
	svc := privservice.NewService(store, files, enq, auditor, clock, time.Hour)

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

	// === 2. ANONYMIZE c1: PII removed, messages masked, integrity kept ===
	if err := svc.Anonymize(ctx, "c1"); err != nil {
		t.Fatalf("anonymize: %v", err)
	}
	var c1 bson.M
	_ = db.Collection("contacts").FindOne(bg, bson.M{"_id": "c1"}).Decode(&c1)
	if c1["name"] != "Contato Anonimizado" || c1["phone"] != "" || c1["document"] != "" || c1["email"] != "" {
		t.Errorf("PII not cleared: %v", c1)
	}
	if ids, ok := c1["identities"].(bson.A); !ok || len(ids) != 0 {
		t.Errorf("channel-identity handles (PII) must be cleared, got %v", c1["identities"])
	}
	if c1["anonymized"] != true {
		t.Errorf("contact must be flagged anonymized: %v", c1["anonymized"])
	}
	if c1["_id"] != "c1" {
		t.Errorf("contact id/integrity lost")
	}
	// Re-anonymizing the same contact is idempotent (no 500, no-op).
	if err := svc.Anonymize(ctx, "c1"); err != nil {
		t.Fatalf("re-anonymize must be idempotent, got: %v", err)
	}
	var m1 bson.M
	_ = db.Collection("messages").FindOne(bg, bson.M{"_id": "m1"}).Decode(&m1)
	mt, _ := m1["text"].(string)
	if strings.Contains(mt, "João Silva") || strings.Contains(mt, "11999998888") || strings.Contains(mt, "joao@x.com") {
		t.Errorf("message PII not masked: %q", mt)
	}
	t.Logf("masked message: %q", mt)

	// === 3. LEGAL HOLD: anonymizing c2 is refused ===
	err = svc.Anonymize(ctx, "c2")
	if err == nil {
		t.Fatalf("expected legal-hold refusal for c2")
	}
	t.Logf("legal-hold refusal: %v", err)
	var c2 bson.M
	_ = db.Collection("contacts").FindOne(bg, bson.M{"_id": "c2"}).Decode(&c2)
	if c2["name"] != "Maria Held" {
		t.Errorf("held contact must not be anonymized: %v", c2)
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
		"privacy.contact.anonymized", "privacy.contact.anonymize_refused",
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
