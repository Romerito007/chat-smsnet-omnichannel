package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func clk() fixedClock {
	tm, _ := time.Parse(time.RFC3339, "2026-06-11T12:00:00Z")
	return fixedClock{t: tm}
}

func ctxT() context.Context { return shared.WithTenant(context.Background(), "t1") }

// fakeStore is an in-memory repository.Store.
type fakeStore struct {
	retention   *entity.RetentionPolicy
	savedRet    *entity.RetentionPolicy
	exports     map[string]*entity.ExportRequest
	bundle      *repository.ExportBundle
	bundleErr   error
	anonymized  *repository.Anonymized
	updatedMsgs map[string]string
	held        bool
	retResult   repository.RetentionResult
	retArg      *entity.RetentionPolicy
}

func newFakeStore() *fakeStore {
	return &fakeStore{exports: map[string]*entity.ExportRequest{}, updatedMsgs: map[string]string{}}
}

func (f *fakeStore) GetRetention(context.Context) (*entity.RetentionPolicy, error) {
	return f.retention, nil
}
func (f *fakeStore) SaveRetention(_ context.Context, p *entity.RetentionPolicy) error {
	f.savedRet = p
	return nil
}
func (f *fakeStore) CreateExport(_ context.Context, e *entity.ExportRequest) error {
	f.exports[e.ID] = e
	return nil
}
func (f *fakeStore) UpdateExport(_ context.Context, e *entity.ExportRequest) error {
	f.exports[e.ID] = e
	return nil
}
func (f *fakeStore) FindExport(_ context.Context, id string) (*entity.ExportRequest, error) {
	e, ok := f.exports[id]
	if !ok {
		return nil, apperror.NotFound("export not found")
	}
	return e, nil
}
func (f *fakeStore) CollectBundle(context.Context, string) (*repository.ExportBundle, error) {
	return f.bundle, f.bundleErr
}
func (f *fakeStore) AnonymizeContact(_ context.Context, _ string, a repository.Anonymized) error {
	f.anonymized = &a
	return nil
}
func (f *fakeStore) UpdateMessageText(_ context.Context, id, text string) error {
	f.updatedMsgs[id] = text
	return nil
}
func (f *fakeStore) HasActiveLegalHold(context.Context, string, time.Time) (bool, error) {
	return f.held, nil
}
func (f *fakeStore) ApplyRetention(_ context.Context, p entity.RetentionPolicy, _ time.Time) (repository.RetentionResult, error) {
	f.retArg = &p
	return f.retResult, nil
}

// fakeFiles is an in-memory contracts.FileStore.
type fakeFiles struct {
	saved map[string][]byte
}

func newFakeFiles() *fakeFiles { return &fakeFiles{saved: map[string][]byte{}} }

func (f *fakeFiles) Save(key string, data []byte) error { f.saved[key] = data; return nil }
func (f *fakeFiles) SignedURL(key string, ttl time.Duration) (string, time.Time, error) {
	return "https://files.example/" + key, time.Now().Add(ttl), nil
}
func (f *fakeFiles) Resolve(token string) (string, error) { return token, nil }
func (f *fakeFiles) Open(key string) ([]byte, string, error) {
	return f.saved[key], "export.json", nil
}

// fakeEnqueuer records the export task.
type fakeEnqueuer struct{ task *contracts.ExportTask }

func (e *fakeEnqueuer) EnqueueExport(t contracts.ExportTask) error { e.task = &t; return nil }

// fakeAuditor records entries.
type fakeAuditor struct{ entries []shared.AuditEntry }

func (a *fakeAuditor) Record(_ context.Context, e shared.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

func (a *fakeAuditor) has(action string) bool {
	for _, e := range a.entries {
		if e.Action == action {
			return true
		}
	}
	return false
}

func newSvc(store *fakeStore, files *fakeFiles, enq *fakeEnqueuer, aud *fakeAuditor) *Service {
	return NewService(store, files, enq, aud, clk(), time.Hour)
}

func TestRequestExport_CreatesEnqueuesAndAudits(t *testing.T) {
	store, files, enq, aud := newFakeStore(), newFakeFiles(), &fakeEnqueuer{}, &fakeAuditor{}
	svc := newSvc(store, files, enq, aud)
	req, err := svc.RequestExport(ctxT(), "c1")
	if err != nil {
		t.Fatalf("request export: %v", err)
	}
	if req.Status != entity.ExportPending {
		t.Errorf("status = %s, want pending", req.Status)
	}
	if enq.task == nil || enq.task.RequestID != req.ID {
		t.Errorf("export not enqueued for request")
	}
	if !aud.has("privacy.export.requested") {
		t.Errorf("request not audited")
	}
}

func TestRunExport_AssemblesFileSignsURLAndAudits(t *testing.T) {
	store, files, enq, aud := newFakeStore(), newFakeFiles(), &fakeEnqueuer{}, &fakeAuditor{}
	store.exports["r1"] = &entity.ExportRequest{ID: "r1", TenantID: "t1", ContactID: "c1", Status: entity.ExportPending}
	store.bundle = &repository.ExportBundle{
		Contact:       repository.ContactData{ID: "c1", Name: "Alice"},
		Conversations: []repository.ConversationData{{ID: "cv1"}},
	}
	svc := newSvc(store, files, enq, aud)
	if err := svc.RunExport(ctxT(), "r1"); err != nil {
		t.Fatalf("run export: %v", err)
	}
	got := store.exports["r1"]
	if got.Status != entity.ExportReady {
		t.Fatalf("status = %s, want ready", got.Status)
	}
	if got.DownloadURL == "" || got.StorageKey == "" {
		t.Errorf("signed URL / storage key not set: %+v", got)
	}
	if len(files.saved[got.StorageKey]) == 0 {
		t.Errorf("file not written")
	}
	if !aud.has("privacy.export.generated") {
		t.Errorf("export generation not audited")
	}
}

func TestAnonymize_RemovesPIIMasksMessagesAndAudits(t *testing.T) {
	store, files, enq, aud := newFakeStore(), newFakeFiles(), &fakeEnqueuer{}, &fakeAuditor{}
	store.bundle = &repository.ExportBundle{
		Contact: repository.ContactData{ID: "c1", Name: "João Silva", Phone: "+5511999998888", Document: "123.456.789-00"},
		Conversations: []repository.ConversationData{{
			ID: "cv1",
			Messages: []repository.MessageData{
				{ID: "m1", Text: "Oi, meu nome é João Silva e meu telefone é +5511999998888"},
				{ID: "m2", Text: "sem pii aqui"},
			},
		}},
	}
	svc := newSvc(store, files, enq, aud)
	if err := svc.Anonymize(ctxT(), "c1"); err != nil {
		t.Fatalf("anonymize: %v", err)
	}
	if store.anonymized == nil || store.anonymized.Phone != "" || store.anonymized.Document != "" {
		t.Errorf("contact PII not cleared: %+v", store.anonymized)
	}
	if store.anonymized.Name != anonymizedName {
		t.Errorf("name not anonymized: %q", store.anonymized.Name)
	}
	masked, ok := store.updatedMsgs["m1"]
	if !ok {
		t.Fatalf("message with PII not masked")
	}
	if strings.Contains(masked, "João Silva") || strings.Contains(masked, "5511999998888") {
		t.Errorf("PII still present after masking: %q", masked)
	}
	if _, touched := store.updatedMsgs["m2"]; touched {
		t.Errorf("clean message should not be rewritten")
	}
	if !aud.has("privacy.contact.anonymized") {
		t.Errorf("anonymization not audited")
	}
}

func TestAnonymize_RefusesUnderLegalHold(t *testing.T) {
	store, files, enq, aud := newFakeStore(), newFakeFiles(), &fakeEnqueuer{}, &fakeAuditor{}
	store.held = true
	svc := newSvc(store, files, enq, aud)
	err := svc.Anonymize(ctxT(), "c1")
	if apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("expected forbidden under legal hold, got %v", err)
	}
	if store.anonymized != nil {
		t.Errorf("must not anonymize a contact under legal hold")
	}
	if !aud.has("privacy.contact.anonymize_refused") {
		t.Errorf("refusal not audited")
	}
}

func TestUpdateRetention_PartialUpdateClampsAndAudits(t *testing.T) {
	store, files, enq, aud := newFakeStore(), newFakeFiles(), &fakeEnqueuer{}, &fakeAuditor{}
	svc := newSvc(store, files, enq, aud)
	msgs := 90
	neg := -5
	p, err := svc.UpdateRetention(ctxT(), contracts.UpdateRetention{MessagesDays: &msgs, AuditLogsDays: &neg})
	if err != nil {
		t.Fatalf("update retention: %v", err)
	}
	if p.MessagesDays != 90 {
		t.Errorf("messages days = %d, want 90", p.MessagesDays)
	}
	if p.AuditLogsDays != 0 {
		t.Errorf("negative value not clamped: %d", p.AuditLogsDays)
	}
	if store.savedRet == nil {
		t.Errorf("policy not saved")
	}
	if !aud.has("privacy.retention.updated") {
		t.Errorf("update not audited")
	}
}

func TestGetRetention_DefaultsWhenUnset(t *testing.T) {
	store, files, enq, aud := newFakeStore(), newFakeFiles(), &fakeEnqueuer{}, &fakeAuditor{}
	svc := newSvc(store, files, enq, aud)
	p, err := svc.GetRetention(ctxT())
	if err != nil {
		t.Fatalf("get retention: %v", err)
	}
	if p.TenantID != "t1" || p.MessagesDays != 0 {
		t.Errorf("expected zero default policy for tenant, got %+v", p)
	}
}

func TestApplyRetention_AuditsOnlyWhenDeleting(t *testing.T) {
	store, files, enq, aud := newFakeStore(), newFakeFiles(), &fakeEnqueuer{}, &fakeAuditor{}
	store.retention = &entity.RetentionPolicy{TenantID: "t1", MessagesDays: 30}
	store.retResult = repository.RetentionResult{Messages: 4}
	svc := newSvc(store, files, enq, aud)
	n, err := svc.ApplyRetention(ctxT())
	if err != nil {
		t.Fatalf("apply retention: %v", err)
	}
	if n != 4 {
		t.Errorf("total deleted = %d, want 4", n)
	}
	if !aud.has("privacy.retention.applied") {
		t.Errorf("retention application not audited")
	}
}

func TestApplyRetention_NoPolicyIsNoOp(t *testing.T) {
	store, files, enq, aud := newFakeStore(), newFakeFiles(), &fakeEnqueuer{}, &fakeAuditor{}
	svc := newSvc(store, files, enq, aud)
	n, err := svc.ApplyRetention(ctxT())
	if err != nil || n != 0 {
		t.Fatalf("expected no-op, got n=%d err=%v", n, err)
	}
	if store.retArg != nil {
		t.Errorf("ApplyRetention must not run without a policy")
	}
}

func TestRequireTenant(t *testing.T) {
	store, files, enq, aud := newFakeStore(), newFakeFiles(), &fakeEnqueuer{}, &fakeAuditor{}
	svc := newSvc(store, files, enq, aud)
	if _, err := svc.GetRetention(context.Background()); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expected forbidden without tenant, got %v", err)
	}
}
