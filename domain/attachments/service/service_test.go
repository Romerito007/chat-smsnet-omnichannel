package service

import (
	"context"
	"testing"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	convcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// fakeAttachRepo is an in-memory attachment store.
type fakeAttachRepo struct{ items map[string]*entity.Attachment }

func newRepo() *fakeAttachRepo { return &fakeAttachRepo{items: map[string]*entity.Attachment{}} }

func (r *fakeAttachRepo) Create(_ context.Context, a *entity.Attachment) error {
	r.items[a.ID] = a
	return nil
}
func (r *fakeAttachRepo) Update(_ context.Context, a *entity.Attachment) error {
	r.items[a.ID] = a
	return nil
}
func (r *fakeAttachRepo) FindByID(_ context.Context, id string) (*entity.Attachment, error) {
	a, ok := r.items[id]
	if !ok {
		return nil, apperror.NotFound("attachment not found")
	}
	return a, nil
}

// fakeConvRepo / fakeMsgRepo satisfy the repository interfaces used by the
// service (only FindByID is exercised).
type fakeConvRepo struct{ conv *conventity.Conversation }

func (r *fakeConvRepo) Create(context.Context, *conventity.Conversation) error { return nil }
func (r *fakeConvRepo) Update(context.Context, *conventity.Conversation) error { return nil }
func (r *fakeConvRepo) FindByID(_ context.Context, id string) (*conventity.Conversation, error) {
	if r.conv == nil || r.conv.ID != id {
		return nil, apperror.NotFound("conversation not found")
	}
	return r.conv, nil
}
func (r *fakeConvRepo) FindOpenByContactChannel(context.Context, string, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nope")
}
func (r *fakeConvRepo) List(context.Context, convcontracts.ListFilter, convcontracts.Visibility, shared.PageRequest) ([]*conventity.Conversation, error) {
	return nil, nil
}
func (r *fakeConvRepo) ListInactiveOpen(context.Context, time.Time, int) ([]*conventity.Conversation, error) {
	return nil, nil
}

type fakeMsgRepo struct{ msg *conventity.Message }

func (r *fakeMsgRepo) Create(context.Context, *conventity.Message) error { return nil }
func (r *fakeMsgRepo) Update(context.Context, *conventity.Message) error { return nil }
func (r *fakeMsgRepo) FindByID(_ context.Context, id string) (*conventity.Message, error) {
	if r.msg == nil || r.msg.ID != id {
		return nil, apperror.NotFound("message not found")
	}
	return r.msg, nil
}
func (r *fakeMsgRepo) ListByConversation(context.Context, string, shared.PageRequest) ([]*conventity.Message, error) {
	return nil, nil
}

// fakeStorage records calls and returns canned targets.
type fakeStorage struct {
	provider  string
	uploadKey string
	putCalls  int
	redirect  string
	missing   bool // when true, Exists reports the object was not uploaded
}

func (s *fakeStorage) Provider() string            { return s.provider }
func (s *fakeStorage) Exists(string) (bool, error) { return !s.missing, nil }
func (s *fakeStorage) SignUpload(key, _ string, _ int64, _ time.Duration) (contracts.UploadTarget, error) {
	s.uploadKey = key
	return contracts.UploadTarget{URL: "http://up/" + key, Method: "PUT"}, nil
}
func (s *fakeStorage) Download(_, filename string, _ time.Duration) (contracts.DownloadResult, error) {
	if s.redirect != "" {
		return contracts.DownloadResult{RedirectURL: s.redirect}, nil
	}
	return contracts.DownloadResult{Data: []byte("filebytes"), Filename: filename}, nil
}
func (s *fakeStorage) Put(string, string, []byte) error { s.putCalls++; return nil }

func ctxAuth(scope authz.SectorScope, sectors []string, userID string) context.Context {
	ctx := shared.WithTenant(context.Background(), "t1")
	return authz.WithAuthContext(ctx, authz.NewAuthContext("t1", userID, authz.AllPermissions(), sectors, scope))
}

func newSvc(repo *fakeAttachRepo, conv *fakeConvRepo, msg *fakeMsgRepo, st *fakeStorage, cfg Config) *Service {
	return NewService(repo, st, conv, msg, fixedClock{time.Now()}, cfg)
}

func TestRequestUploadURL_ValidatesAndReserves(t *testing.T) {
	repo := newRepo()
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	st := &fakeStorage{provider: "local"}
	svc := newSvc(repo, conv, &fakeMsgRepo{}, st, Config{MaxSizeBytes: 1000, AllowedContentTypes: []string{"image/*"}})

	att, target, err := svc.RequestUploadURL(ctxAuth(authz.ScopeAll, nil, "u1"), contracts.CreateUploadURL{
		ConversationID: "cv1", Filename: "pic.png", ContentType: "image/png", Size: 500,
	})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if att.Status != entity.StatusPending || att.StorageProvider != "local" {
		t.Errorf("unexpected attachment: %+v", att)
	}
	if target.URL == "" || target.Method != "PUT" {
		t.Errorf("bad target: %+v", target)
	}
	if repo.items[att.ID] == nil {
		t.Errorf("attachment not persisted")
	}
}

func TestRequestUploadURL_RejectsBadTypeAndSize(t *testing.T) {
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	svc := newSvc(newRepo(), conv, &fakeMsgRepo{}, &fakeStorage{provider: "local"}, Config{MaxSizeBytes: 100, AllowedContentTypes: []string{"image/png"}})

	_, _, err := svc.RequestUploadURL(ctxAuth(authz.ScopeAll, nil, "u1"), contracts.CreateUploadURL{
		ConversationID: "cv1", Filename: "x.exe", ContentType: "application/octet-stream", Size: 50,
	})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation error for disallowed type, got %v", err)
	}

	_, _, err = svc.RequestUploadURL(ctxAuth(authz.ScopeAll, nil, "u1"), contracts.CreateUploadURL{
		ConversationID: "cv1", Filename: "x.png", ContentType: "image/png", Size: 5000,
	})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation error for oversize, got %v", err)
	}
}

func TestRequestUploadURL_EnforcesConversationAccess(t *testing.T) {
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s9"}}
	svc := newSvc(newRepo(), conv, &fakeMsgRepo{}, &fakeStorage{provider: "local"}, Config{})
	// Scope own, different sector, not assigned → not visible.
	_, _, err := svc.RequestUploadURL(ctxAuth(authz.ScopeOwn, []string{"s1"}, "u1"), contracts.CreateUploadURL{
		ConversationID: "cv1", Filename: "x.png", ContentType: "image/png", Size: 10,
	})
	if apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("expected not found for inaccessible conversation, got %v", err)
	}
}

func TestConfirm_LinksMessageAndMarksReady(t *testing.T) {
	repo := newRepo()
	repo.items["a1"] = &entity.Attachment{ID: "a1", TenantID: "t1", ConversationID: "cv1", Status: entity.StatusPending}
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	msg := &fakeMsgRepo{msg: &conventity.Message{ID: "m1", ConversationID: "cv1"}}
	svc := newSvc(repo, conv, msg, &fakeStorage{provider: "local"}, Config{DownloadBaseURL: "http://api"})

	att, err := svc.Confirm(ctxAuth(authz.ScopeAll, nil, "u1"), contracts.ConfirmUpload{AttachmentID: "a1", MessageID: "m1"})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if att.Status != entity.StatusReady {
		t.Errorf("status = %s, want ready", att.Status)
	}
	if att.MessageID != "m1" {
		t.Errorf("message not linked: %q", att.MessageID)
	}
	if att.SignedURL != "http://api/v1/attachments/a1/download" {
		t.Errorf("unexpected download url: %q", att.SignedURL)
	}
}

func TestConfirm_RejectsWhenObjectNotUploaded(t *testing.T) {
	repo := newRepo()
	repo.items["a1"] = &entity.Attachment{ID: "a1", TenantID: "t1", ConversationID: "cv1", Status: entity.StatusPending}
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	svc := newSvc(repo, conv, &fakeMsgRepo{}, &fakeStorage{provider: "s3", missing: true}, Config{})

	_, err := svc.Confirm(ctxAuth(authz.ScopeAll, nil, "u1"), contracts.ConfirmUpload{AttachmentID: "a1"})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("confirm must reject when the object was never uploaded, got %v", err)
	}
	if repo.items["a1"].Status == entity.StatusReady {
		t.Error("attachment must not be marked ready when the upload is missing")
	}
}

func TestConfirm_RejectsForeignMessage(t *testing.T) {
	repo := newRepo()
	repo.items["a1"] = &entity.Attachment{ID: "a1", TenantID: "t1", ConversationID: "cv1", Status: entity.StatusPending}
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	msg := &fakeMsgRepo{msg: &conventity.Message{ID: "m1", ConversationID: "OTHER"}}
	svc := newSvc(repo, conv, msg, &fakeStorage{provider: "local"}, Config{})
	_, err := svc.Confirm(ctxAuth(authz.ScopeAll, nil, "u1"), contracts.ConfirmUpload{AttachmentID: "a1", MessageID: "m1"})
	if apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected validation error linking foreign message, got %v", err)
	}
}

func TestDownload_ChecksAccessAndServes(t *testing.T) {
	repo := newRepo()
	repo.items["a1"] = &entity.Attachment{ID: "a1", TenantID: "t1", ConversationID: "cv1", Status: entity.StatusReady, ContentType: "image/png", Filename: "p.png"}
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	svc := newSvc(repo, conv, &fakeMsgRepo{}, &fakeStorage{provider: "local"}, Config{})

	res, err := svc.Download(ctxAuth(authz.ScopeAll, nil, "u1"), "a1")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if string(res.Data) != "filebytes" || res.ContentType != "image/png" {
		t.Errorf("unexpected download: %+v", res)
	}

	// Inaccessible conversation → not found.
	_, err = svc.Download(ctxAuth(authz.ScopeOwn, []string{"other"}, "u9"), "a1")
	if apperror.From(err).Code != apperror.CodeNotFound {
		t.Errorf("expected not found for inaccessible download, got %v", err)
	}
}

func TestDownload_PendingIsConflict(t *testing.T) {
	repo := newRepo()
	repo.items["a1"] = &entity.Attachment{ID: "a1", TenantID: "t1", ConversationID: "cv1", Status: entity.StatusPending}
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	svc := newSvc(repo, conv, &fakeMsgRepo{}, &fakeStorage{provider: "local"}, Config{})
	_, err := svc.Download(ctxAuth(authz.ScopeAll, nil, "u1"), "a1")
	if apperror.From(err).Code != apperror.CodeConflict {
		t.Errorf("expected conflict for unconfirmed attachment, got %v", err)
	}
}

func TestRequireTenant(t *testing.T) {
	svc := newSvc(newRepo(), &fakeConvRepo{}, &fakeMsgRepo{}, &fakeStorage{provider: "local"}, Config{})
	_, err := svc.Get(context.Background(), "a1")
	if apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("expected forbidden without tenant, got %v", err)
	}
}
