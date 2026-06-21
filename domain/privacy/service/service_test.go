package service

import (
	"context"
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
	held        bool
	retResult   repository.RetentionResult
	retArg      *entity.RetentionPolicy
	linkedDeals []repository.DealLink
	eraseResult repository.EraseResult
	erasedID    string
	eraseForced *bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{exports: map[string]*entity.ExportRequest{}}
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
func (f *fakeStore) LinkedDeals(context.Context, string) ([]repository.DealLink, error) {
	return f.linkedDeals, nil
}
func (f *fakeStore) EraseContact(_ context.Context, contactID string, unlinkDeals bool) (repository.EraseResult, error) {
	f.erasedID = contactID
	f.eraseForced = &unlinkDeals
	return f.eraseResult, nil
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
func (f *fakeFiles) Delete(key string) error { delete(f.saved, key); return nil }

// fakeBlobs is an in-memory contracts.BlobStore recording deleted keys.
type fakeBlobs struct{ deleted []string }

func (b *fakeBlobs) Delete(key string) error { b.deleted = append(b.deleted, key); return nil }

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
	return NewService(store, files, &fakeBlobs{}, enq, aud, clk(), time.Hour)
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

func TestEraseContact_DeletesUnlinksPurgesAndAudits(t *testing.T) {
	store, files, enq, aud := newFakeStore(), newFakeFiles(), &fakeEnqueuer{}, &fakeAuditor{}
	blobs := &fakeBlobs{}
	store.eraseResult = repository.EraseResult{
		Conversations: 2, Messages: 9, DealsUnlinked: 1,
		BlobKeys:   []string{"att/a1", "att/a2"},
		ExportKeys: []string{"exports/t1/c1/r1.json"},
	}
	files.saved["exports/t1/c1/r1.json"] = []byte("{}")
	svc := NewService(store, files, blobs, enq, aud, clk(), time.Hour)

	if err := svc.EraseContact(ctxT(), "c1", false); err != nil {
		t.Fatalf("erase: %v", err)
	}
	if store.erasedID != "c1" || store.eraseForced == nil {
		t.Fatalf("EraseContact not called for the contact")
	}
	if len(blobs.deleted) != 2 {
		t.Errorf("attachment blobs not purged: %v", blobs.deleted)
	}
	if _, still := files.saved["exports/t1/c1/r1.json"]; still {
		t.Errorf("export bundle not purged")
	}
	if !aud.has("privacy.contact.erased") {
		t.Errorf("erasure not audited")
	}
}

func TestEraseContact_RefusesUnderLegalHold(t *testing.T) {
	store, files, enq, aud := newFakeStore(), newFakeFiles(), &fakeEnqueuer{}, &fakeAuditor{}
	store.held = true
	svc := newSvc(store, files, enq, aud)
	err := svc.EraseContact(ctxT(), "c1", true)
	if apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("expected forbidden under legal hold, got %v", err)
	}
	if store.erasedID != "" {
		t.Errorf("must not erase a contact under legal hold")
	}
	if !aud.has("privacy.contact.erase_refused") {
		t.Errorf("refusal not audited")
	}
}

func TestEraseContact_BlocksOnLinkedDealsWithoutForce(t *testing.T) {
	store, files, enq, aud := newFakeStore(), newFakeFiles(), &fakeEnqueuer{}, &fakeAuditor{}
	store.linkedDeals = []repository.DealLink{{ID: "d1", Title: "Big deal"}}
	svc := newSvc(store, files, enq, aud)

	err := svc.EraseContact(ctxT(), "c1", false)
	appErr := apperror.From(err)
	if appErr.Code != apperror.CodeConflict {
		t.Fatalf("expected conflict on linked deals, got %v", err)
	}
	if appErr.Details["linked_deals"] == nil {
		t.Errorf("409 must surface the linked deals: %+v", appErr.Details)
	}
	if store.erasedID != "" {
		t.Errorf("a blocked erase must delete nothing")
	}
	if !aud.has("privacy.contact.erase_blocked") {
		t.Errorf("block not audited")
	}
}

func TestEraseContact_ForceUnlinksLinkedDealsAndProceeds(t *testing.T) {
	store, files, enq, aud := newFakeStore(), newFakeFiles(), &fakeEnqueuer{}, &fakeAuditor{}
	store.linkedDeals = []repository.DealLink{{ID: "d1", Title: "Big deal"}}
	svc := newSvc(store, files, enq, aud)

	if err := svc.EraseContact(ctxT(), "c1", true); err != nil {
		t.Fatalf("forced erase: %v", err)
	}
	if store.erasedID != "c1" || store.eraseForced == nil || !*store.eraseForced {
		t.Errorf("forced erase must pass unlinkDeals=true to the store")
	}
	if !aud.has("privacy.contact.erased") {
		t.Errorf("erasure not audited")
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
