package service

import (
	"context"
	"errors"
	"strings"
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
func (r *fakeAttachRepo) FindByIDs(_ context.Context, ids []string) ([]*entity.Attachment, error) {
	var out []*entity.Attachment
	for _, id := range ids {
		if a, ok := r.items[id]; ok {
			out = append(out, a)
		}
	}
	return out, nil
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
func (r *fakeConvRepo) FindByIDs(context.Context, []string) ([]*conventity.Conversation, error) {
	return nil, nil
}
func (r *fakeConvRepo) FindLastByContactChannelID(context.Context, string, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindOpenByContactChannelID(context.Context, string, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nope")
}
func (r *fakeConvRepo) FindOpenByContact(context.Context, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
}
func (r *fakeConvRepo) FindLastByContact(context.Context, string) (*conventity.Conversation, error) {
	return nil, apperror.NotFound("nf")
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
func (r *fakeMsgRepo) LatestByConversation(context.Context, string) (*conventity.Message, error) {
	return nil, apperror.NotFound("none")
}
func (r *fakeMsgRepo) LatestByConversations(context.Context, []string) (map[string]*conventity.Message, error) {
	return nil, nil
}

// fakeStorage records calls and returns canned targets.
type fakeStorage struct {
	provider  string
	uploadKey string
	putCalls  int
	redirect  string
	missing   bool              // when true, Exists reports the object was not uploaded
	objects   map[string][]byte // Put/Get/Delete object store
	getData   []byte            // canned Get bytes when objects has no entry
	deleted   []string          // keys passed to Delete
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
func (s *fakeStorage) Put(key, _ string, data []byte) error {
	s.putCalls++
	if s.objects == nil {
		s.objects = map[string][]byte{}
	}
	s.objects[key] = data
	return nil
}
func (s *fakeStorage) Get(key string) ([]byte, error) {
	if s.objects != nil {
		if b, ok := s.objects[key]; ok {
			return b, nil
		}
	}
	if s.getData != nil {
		return s.getData, nil
	}
	return []byte("storedbytes"), nil
}
func (s *fakeStorage) Delete(key string) error {
	s.deleted = append(s.deleted, key)
	if s.objects != nil {
		delete(s.objects, key)
	}
	return nil
}

func ctxAuth(scope authz.SectorScope, sectors []string, userID string) context.Context {
	ctx := shared.WithTenant(context.Background(), "t1")
	return authz.WithAuthContext(ctx, authz.NewAuthContext("t1", userID, authz.AllPermissions(), sectors, scope))
}

func newSvc(repo *fakeAttachRepo, conv *fakeConvRepo, msg *fakeMsgRepo, st *fakeStorage, cfg Config) *Service {
	return NewService(repo, st, conv, msg, fixedClock{time.Now()}, cfg)
}

// An avatar upload needs no conversation: it is tenant-scoped, namespaced under
// avatars/, restricted to image/* and confirm/download skip the conversation check.
func TestRequestUploadURL_AvatarNoConversation(t *testing.T) {
	repo := newRepo()
	st := &fakeStorage{provider: "s3"}
	svc := newSvc(repo, &fakeConvRepo{}, &fakeMsgRepo{}, st,
		Config{AvatarMaxSizeBytes: 5 << 20, DownloadBaseURL: "http://api"})
	// A restricted agent (own scope, no sectors) — would fail a conversation check.
	ctx := ctxAuth(authz.ScopeOwn, nil, "u1")

	att, target, err := svc.RequestUploadURL(ctx, contracts.CreateUploadURL{
		Filename: "face.png", ContentType: "image/png", Size: 1000,
		Avatar: &contracts.AvatarTarget{OwnerType: "contacts", OwnerID: "c1"},
	})
	if err != nil {
		t.Fatalf("avatar upload-url: %v", err)
	}
	if att.ConversationID != "" {
		t.Errorf("avatar attachment must have no conversation, got %q", att.ConversationID)
	}
	wantKey := "avatars/t1/contacts/c1/face.png"
	if st.uploadKey != wantKey || target.URL == "" {
		t.Errorf("avatar key = %q, want %q", st.uploadKey, wantKey)
	}

	// Confirm + download work without any conversation visibility.
	if _, err := svc.Confirm(ctx, contracts.ConfirmUpload{AttachmentID: att.ID}); err != nil {
		t.Fatalf("confirm avatar: %v", err)
	}
	if repo.items[att.ID].Status != entity.StatusReady {
		t.Errorf("avatar not marked ready")
	}
	if _, err := svc.Download(ctx, att.ID); err != nil {
		t.Errorf("download avatar: %v", err)
	}
}

// Avatars must be images within the avatar size limit.
func TestRequestUploadURL_AvatarRejectsNonImageAndOversize(t *testing.T) {
	svc := newSvc(newRepo(), &fakeConvRepo{}, &fakeMsgRepo{}, &fakeStorage{provider: "s3"},
		Config{AvatarMaxSizeBytes: 1000})
	ctx := ctxAuth(authz.ScopeAll, nil, "u1")

	if _, _, err := svc.RequestUploadURL(ctx, contracts.CreateUploadURL{
		Filename: "x.pdf", ContentType: "application/pdf", Size: 100,
		Avatar: &contracts.AvatarTarget{OwnerType: "contacts", OwnerID: "c1"},
	}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("non-image avatar must be rejected, got %v", err)
	}
	if _, _, err := svc.RequestUploadURL(ctx, contracts.CreateUploadURL{
		Filename: "big.png", ContentType: "image/png", Size: 5000,
		Avatar: &contracts.AvatarTarget{OwnerType: "contacts", OwnerID: "c1"},
	}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("oversize avatar must be rejected, got %v", err)
	}
	if _, _, err := svc.RequestUploadURL(ctx, contracts.CreateUploadURL{
		Filename: "x.png", ContentType: "image/png", Size: 100,
		Avatar: &contracts.AvatarTarget{OwnerType: "robots", OwnerID: "c1"},
	}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("invalid owner type must be rejected, got %v", err)
	}
}

// HydrateAttachments fills full media metadata + a JWT-gated download URL, and
// ValidateMessageAttachments rejects unknown/unconfirmed ids.
func TestHydrateAndValidateMessageAttachments(t *testing.T) {
	repo := newRepo()
	svc := newSvc(repo, &fakeConvRepo{}, &fakeMsgRepo{}, &fakeStorage{provider: "s3"},
		Config{DownloadBaseURL: "http://api"})
	ctx := ctxAuth(authz.ScopeAll, nil, "u1")

	repo.items["a1"] = &entity.Attachment{ID: "a1", TenantID: "t1", ContentType: "image/jpeg", Filename: "p.jpg", Size: 1200, Status: entity.StatusReady}
	repo.items["a2"] = &entity.Attachment{ID: "a2", TenantID: "t1", ContentType: "audio/mpeg", Filename: "v.mp3", Size: 900, Status: entity.StatusReady}
	repo.items["pending"] = &entity.Attachment{ID: "pending", TenantID: "t1", ContentType: "image/png", Status: entity.StatusPending}

	got, err := svc.HydrateAttachments(ctx, []string{"a1", "a2", "missing"})
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	a1 := got["a1"]
	// The rendered URL is the signed, JWT-less channel-media URL so the dashboard
	// loads it directly in <img src> (no Authorization, no per-image access check).
	if !strings.HasPrefix(a1.URL, "http://api/v1/channel-media/") || a1.ContentType != "image/jpeg" || a1.Filename != "p.jpg" || a1.Size != 1200 {
		t.Errorf("a1 not hydrated with a signed media URL: %+v", a1)
	}
	if _, ok := got["missing"]; ok {
		t.Errorf("missing id should be absent from the map")
	}

	if err := svc.ValidateMessageAttachments(ctx, []string{"a1", "a2"}); err != nil {
		t.Errorf("ready ids must validate: %v", err)
	}
	if apperror.From(svc.ValidateMessageAttachments(ctx, []string{"a1", "missing"})).Code != apperror.CodeValidation {
		t.Errorf("unknown id must be a validation error")
	}
	if apperror.From(svc.ValidateMessageAttachments(ctx, []string{"pending"})).Code != apperror.CodeValidation {
		t.Errorf("unconfirmed id must be a validation error")
	}
}

// Confirm preserves the metadata captured at upload-url and never leaves a ready
// attachment with an empty content_type.
func TestConfirm_PreservesMetadataAndContentType(t *testing.T) {
	repo := newRepo()
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	svc := newSvc(repo, conv, &fakeMsgRepo{}, &fakeStorage{provider: "s3"}, Config{DownloadBaseURL: "http://api"})
	ctx := ctxAuth(authz.ScopeAll, nil, "u1")

	repo.items["a1"] = &entity.Attachment{ID: "a1", TenantID: "t1", ConversationID: "cv1", ContentType: "image/png", Filename: "p.png", Size: 50, Status: entity.StatusPending}
	att, err := svc.Confirm(ctx, contracts.ConfirmUpload{AttachmentID: "a1"})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if att.Status != entity.StatusReady || att.ContentType != "image/png" || att.Filename != "p.png" || att.Size != 50 {
		t.Errorf("confirm lost metadata: %+v", att)
	}

	// A record that somehow has no content_type defaults instead of staying empty.
	repo.items["a2"] = &entity.Attachment{ID: "a2", TenantID: "t1", ConversationID: "cv1", Status: entity.StatusPending}
	att2, err := svc.Confirm(ctx, contracts.ConfirmUpload{AttachmentID: "a2"})
	if err != nil {
		t.Fatalf("confirm a2: %v", err)
	}
	if att2.ContentType == "" {
		t.Errorf("ready attachment must never have empty content_type")
	}
}

// SignedAvatarURLs resolves only ready image avatars, in one batch, to a signed
// /v1/channel-media URL openable WITHOUT a JWT (verified via DownloadSigned).
func TestSignedAvatarURLs_BatchAndJWTLess(t *testing.T) {
	repo := newRepo()
	svc := newSvc(repo, &fakeConvRepo{}, &fakeMsgRepo{}, &fakeStorage{provider: "local"},
		Config{DownloadBaseURL: "http://api", SigningSecret: "s3cr3t", AvatarURLTTL: time.Minute})
	ctx := ctxAuth(authz.ScopeAll, nil, "u1")

	repo.items["av1"] = &entity.Attachment{ID: "av1", TenantID: "t1", StorageKey: "avatars/t1/contacts/c1/p.png", ContentType: "image/png", Filename: "p.png", Status: entity.StatusReady}
	repo.items["nonimg"] = &entity.Attachment{ID: "nonimg", TenantID: "t1", StorageKey: "k", ContentType: "application/pdf", Status: entity.StatusReady}
	repo.items["pending"] = &entity.Attachment{ID: "pending", TenantID: "t1", StorageKey: "k", ContentType: "image/png", Status: entity.StatusPending}

	urls, err := svc.SignedAvatarURLs(ctx, []string{"av1", "nonimg", "pending", "missing"})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if len(urls) != 1 || urls["av1"] == "" {
		t.Fatalf("only the ready image must resolve: %v", urls)
	}
	if !strings.Contains(urls["av1"], "/v1/channel-media/") {
		t.Errorf("avatar URL must use the signed channel-media route: %q", urls["av1"])
	}
	// Openable with no JWT: the token verifies and serves bytes.
	token := urls["av1"][strings.LastIndex(urls["av1"], "/")+1:]
	if _, err := svc.DownloadSigned(token); err != nil {
		t.Errorf("avatar URL must be openable without a JWT: %v", err)
	}
}

// ValidateReadyImage is the gate contacts use before storing an avatar id.
func TestValidateReadyImage(t *testing.T) {
	repo := newRepo()
	svc := newSvc(repo, &fakeConvRepo{}, &fakeMsgRepo{}, &fakeStorage{provider: "s3"}, Config{})
	ctx := ctxAuth(authz.ScopeAll, nil, "u1")

	repo.items["ready-img"] = &entity.Attachment{ID: "ready-img", TenantID: "t1", ContentType: "image/png", Status: entity.StatusReady}
	repo.items["pending-img"] = &entity.Attachment{ID: "pending-img", TenantID: "t1", ContentType: "image/png", Status: entity.StatusPending}
	repo.items["ready-pdf"] = &entity.Attachment{ID: "ready-pdf", TenantID: "t1", ContentType: "application/pdf", Status: entity.StatusReady}

	if err := svc.ValidateReadyImage(ctx, ""); err != nil {
		t.Errorf("empty id (clear avatar) must be allowed: %v", err)
	}
	if err := svc.ValidateReadyImage(ctx, "ready-img"); err != nil {
		t.Errorf("ready image must pass: %v", err)
	}
	for _, id := range []string{"missing", "pending-img", "ready-pdf"} {
		if apperror.From(svc.ValidateReadyImage(ctx, id)).Code != apperror.CodeValidation {
			t.Errorf("%s must be a validation error", id)
		}
	}
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

// Empty AllowedContentTypes means "any" (per the config contract): every content
// type is accepted, while the size limit still applies.
func TestRequestUploadURL_EmptyAllowlistAcceptsAnyType(t *testing.T) {
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	svc := newSvc(newRepo(), conv, &fakeMsgRepo{}, &fakeStorage{provider: "s3"},
		Config{MaxSizeBytes: 26214400}) // AllowedContentTypes nil = any

	for _, ct := range []string{"image/jpeg", "video/mp4", "audio/mpeg", "application/pdf"} {
		_, target, err := svc.RequestUploadURL(ctxAuth(authz.ScopeAll, nil, "u1"), contracts.CreateUploadURL{
			ConversationID: "cv1", Filename: "f", ContentType: ct, Size: 500,
		})
		if err != nil {
			t.Errorf("empty allowlist should accept %q, got %v", ct, err)
		}
		if target.URL == "" {
			t.Errorf("expected an upload url for %q", ct)
		}
	}

	// The size limit still applies even with an empty allowlist.
	if _, _, err := svc.RequestUploadURL(ctxAuth(authz.ScopeAll, nil, "u1"), contracts.CreateUploadURL{
		ConversationID: "cv1", Filename: "big", ContentType: "image/jpeg", Size: 26214401,
	}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("expected oversize rejection, got %v", err)
	}
}

// A wildcard subtype (image/*) matches its types and rejects others.
func TestRequestUploadURL_WildcardSubtype(t *testing.T) {
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	svc := newSvc(newRepo(), conv, &fakeMsgRepo{}, &fakeStorage{provider: "s3"},
		Config{MaxSizeBytes: 1000, AllowedContentTypes: []string{"image/*"}})

	if _, _, err := svc.RequestUploadURL(ctxAuth(authz.ScopeAll, nil, "u1"), contracts.CreateUploadURL{
		ConversationID: "cv1", Filename: "p.jpg", ContentType: "image/jpeg", Size: 100,
	}); err != nil {
		t.Errorf("image/* should accept image/jpeg, got %v", err)
	}
	if _, _, err := svc.RequestUploadURL(ctxAuth(authz.ScopeAll, nil, "u1"), contracts.CreateUploadURL{
		ConversationID: "cv1", Filename: "v.mp4", ContentType: "video/mp4", Size: 100,
	}); apperror.From(err).Code != apperror.CodeValidation {
		t.Errorf("image/* should reject video/mp4, got %v", err)
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
	if !strings.HasPrefix(att.SignedURL, "http://api/v1/channel-media/") {
		t.Errorf("confirm must return a signed, renderable media URL, got: %q", att.SignedURL)
	}
}

func TestIntegrationMediaURL_SignedRoundTrip(t *testing.T) {
	repo := newRepo()
	repo.items["a1"] = &entity.Attachment{
		ID: "a1", TenantID: "t1", ConversationID: "cv1", Status: entity.StatusReady,
		StorageKey: "attachments/t1/cv1/a1/nota.ogg", ContentType: "audio/ogg", Filename: "nota.ogg",
	}
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	svc := newSvc(repo, conv, &fakeMsgRepo{}, &fakeStorage{provider: "local"},
		Config{DownloadBaseURL: "http://api", SigningSecret: "s3cr3t"})

	url, err := svc.IntegrationMediaURL(ctxAuth(authz.ScopeAll, nil, "u1"), "a1")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	const prefix = "http://api/v1/channel-media/"
	if !strings.HasPrefix(url, prefix) {
		t.Fatalf("integration url = %q, want %s...", url, prefix)
	}
	token := strings.TrimPrefix(url, prefix)

	// Valid token serves the bytes with NO JWT and NO tenant context.
	res, err := svc.DownloadSigned(token)
	if err != nil {
		t.Fatalf("download signed: %v", err)
	}
	if string(res.Data) != "filebytes" || res.ContentType != "audio/ogg" {
		t.Errorf("unexpected download: data=%q content_type=%q", res.Data, res.ContentType)
	}
	// A tampered signature is rejected.
	if _, err := svc.DownloadSigned(token + "x"); apperror.From(err).Code != apperror.CodeForbidden {
		t.Errorf("tampered token must be forbidden, got %v", err)
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

// TestStoreInbound_ReturnsSignedRenderableURL is the A2 round trip: an inbound
// media attachment is returned with a signed, JWT-less channel-media URL (so the
// dashboard renders it directly in <img>/<audio> src), and that URL opens via
// DownloadSigned WITHOUT any JWT or tenant context (bearer token).
func TestStoreInbound_ReturnsSignedRenderableURL(t *testing.T) {
	repo := newRepo()
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	svc := newSvc(repo, conv, &fakeMsgRepo{}, &fakeStorage{provider: "s3"},
		Config{DownloadBaseURL: "http://api", SigningSecret: "s3cr3t"})
	ctx := ctxAuth(authz.ScopeAll, nil, "u1")

	att, err := svc.StoreInbound(ctx, "cv1", "3EB0.jpg", "image/jpeg", []byte("\xff\xd8\xff\xe0jpeg"))
	if err != nil {
		t.Fatalf("store inbound: %v", err)
	}
	const prefix = "http://api/v1/channel-media/"
	if !strings.HasPrefix(att.URL, prefix) {
		t.Fatalf("inbound media URL must be the signed channel-media URL, got %q", att.URL)
	}
	// The token opens with NO JWT / NO tenant context (bearer), like the webhook rail.
	token := strings.TrimPrefix(att.URL, prefix)
	if _, err := svc.DownloadSigned(token); err != nil {
		t.Errorf("signed media URL must open without a JWT, got: %v", err)
	}
}

func (r *fakeMsgRepo) FindByExternalMessageID(context.Context, string, string) (*conventity.Message, error) {
	return nil, apperror.NotFound("nf")
}

// mediaExtension maps content types to the URL suffix integrations infer the type
// from, normalizing the type (params after ";" stripped, trimmed, lowercased).
func TestMediaExtension(t *testing.T) {
	cases := map[string]string{
		"audio/ogg":                ".ogg",
		"audio/ogg; codecs=opus":   ".ogg", // normalize away the codecs parameter
		"audio/mpeg":               ".mp3",
		"audio/mp4":                ".m4a",
		"audio/m4a":                ".m4a",
		"audio/aac":                ".aac",
		"audio/amr":                ".amr",
		"audio/wav":                ".wav",
		"image/jpeg":               ".jpg",
		"image/png":                ".png",
		"image/webp":               ".webp",
		"video/mp4":                ".mp4",
		"application/pdf":          ".pdf",
		"  AUDIO/OGG ":             ".ogg", // trim + lowercase
		"application/octet-stream": "",     // unknown → no extension
		"":                         "",
	}
	for ct, want := range cases {
		if got := mediaExtension(ct); got != want {
			t.Errorf("mediaExtension(%q) = %q, want %q", ct, got, want)
		}
	}
}

// The generated media URL ends with the extension derived from the content type
// (one per family), and an unknown type appends nothing.
func TestMediaURL_AppendsExtension(t *testing.T) {
	svc := newSvc(newRepo(), &fakeConvRepo{}, &fakeMsgRepo{}, &fakeStorage{provider: "s3"},
		Config{DownloadBaseURL: "http://api", SigningSecret: "s3cr3t", MediaURLTTL: time.Minute})

	for ct, suffix := range map[string]string{
		"audio/ogg; codecs=opus": ".ogg",
		"image/jpeg":             ".jpg",
		"video/mp4":              ".mp4",
		"application/pdf":        ".pdf",
	} {
		url := svc.mediaURL("k", ct, "f")
		if !strings.HasPrefix(url, "http://api/v1/channel-media/") {
			t.Errorf("url prefix wrong for %q: %q", ct, url)
		}
		if !strings.HasSuffix(url, suffix) {
			t.Errorf("url for %q must end with %q: %q", ct, suffix, url)
		}
	}

	// Unknown content type → no cosmetic extension (current behavior preserved).
	url := svc.mediaURL("k", "application/zip", "f")
	for _, ext := range knownMediaExtensions {
		if strings.HasSuffix(url, ext) {
			t.Errorf("unknown content type must not append an extension: %q", url)
		}
	}
}

// DownloadSigned accepts the token both WITH and WITHOUT the cosmetic extension and
// resolves the same object — the extension is not part of the signed material.
func TestDownloadSigned_TokenWithAndWithoutExtension(t *testing.T) {
	svc := newSvc(newRepo(), &fakeConvRepo{}, &fakeMsgRepo{}, &fakeStorage{provider: "local"},
		Config{DownloadBaseURL: "http://api", SigningSecret: "s3cr3t", MediaURLTTL: time.Minute})

	url := svc.mediaURL("audios/t1/a.ogg", "audio/ogg", "a.ogg")
	tokenWithExt := url[strings.LastIndex(url, "/")+1:]
	if !strings.HasSuffix(tokenWithExt, ".ogg") {
		t.Fatalf("expected the URL to carry the .ogg extension: %q", tokenWithExt)
	}
	tokenNoExt := strings.TrimSuffix(tokenWithExt, ".ogg")

	withExt, err := svc.DownloadSigned(tokenWithExt)
	if err != nil {
		t.Fatalf("download with extension: %v", err)
	}
	noExt, err := svc.DownloadSigned(tokenNoExt)
	if err != nil {
		t.Fatalf("download without extension: %v", err)
	}
	if withExt.ContentType != noExt.ContentType || withExt.ContentType != "audio/ogg" {
		t.Errorf("both forms must resolve the same object: %q vs %q", withExt.ContentType, noExt.ContentType)
	}
}

// fakeAudioConverter is a stand-in for the ffmpeg converter.
type fakeAudioConverter struct {
	called    int
	out       []byte
	reencoded bool
	err       error
}

func (f *fakeAudioConverter) ToOgg(_ context.Context, in []byte) ([]byte, bool, error) {
	f.called++
	if f.err != nil {
		return nil, false, f.err
	}
	out := f.out
	if out == nil {
		out = []byte("OGG" + string(in))
	}
	return out, f.reencoded, nil
}

// (a) A WebM/Opus voice note is remuxed to Ogg/Opus on confirm: content_type,
// filename, size and storage_key all reflect the .ogg result, and the media URL
// (which appends an extension per content_type) ends in .ogg.
func TestConfirm_RemuxesWebmAudioToOgg(t *testing.T) {
	repo := newRepo()
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	st := &fakeStorage{provider: "s3", getData: []byte("webm-opus-bytes")}
	svc := newSvc(repo, conv, &fakeMsgRepo{}, st,
		Config{DownloadBaseURL: "http://api", SigningSecret: "s3cr3t", MediaURLTTL: time.Minute})
	conviter := &fakeAudioConverter{out: []byte("OGGDATA")}
	svc.SetAudioConverter(conviter)
	ctx := ctxAuth(authz.ScopeAll, nil, "u1")

	repo.items["a1"] = &entity.Attachment{
		ID: "a1", TenantID: "t1", ConversationID: "cv1",
		ContentType: "audio/webm;codecs=opus", Filename: "voice.webm",
		StorageKey: "attachments/t1/cv1/a1/voice.webm", Size: 999, Status: entity.StatusPending,
	}
	att, err := svc.Confirm(ctx, contracts.ConfirmUpload{AttachmentID: "a1"})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if conviter.called != 1 {
		t.Fatalf("converter must be invoked once, got %d", conviter.called)
	}
	if att.ContentType != "audio/ogg" {
		t.Errorf("content_type = %q, want audio/ogg", att.ContentType)
	}
	if att.Filename != "voice.ogg" {
		t.Errorf("filename = %q, want voice.ogg", att.Filename)
	}
	if att.StorageKey != "attachments/t1/cv1/a1/voice.ogg" {
		t.Errorf("storage_key = %q, want .../voice.ogg", att.StorageKey)
	}
	if att.Size != int64(len("OGGDATA")) {
		t.Errorf("size = %d, want %d", att.Size, len("OGGDATA"))
	}
	// The new object was stored and the original webm removed.
	if _, ok := st.objects["attachments/t1/cv1/a1/voice.ogg"]; !ok {
		t.Errorf("the .ogg object must be stored")
	}
	if len(st.deleted) != 1 || st.deleted[0] != "attachments/t1/cv1/a1/voice.webm" {
		t.Errorf("the original webm must be deleted, got %v", st.deleted)
	}
	// The signed media URL ends in .ogg (channelMediaURL appends per content_type).
	if !strings.HasSuffix(att.SignedURL, ".ogg") {
		t.Errorf("media URL must end with .ogg, got %q", att.SignedURL)
	}
}

// (b) Non-webm audio (and any other type) passes through untouched.
func TestConfirm_NonWebmAudioPassesThrough(t *testing.T) {
	repo := newRepo()
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	st := &fakeStorage{provider: "s3"}
	svc := newSvc(repo, conv, &fakeMsgRepo{}, st, Config{DownloadBaseURL: "http://api"})
	conviter := &fakeAudioConverter{}
	svc.SetAudioConverter(conviter)
	ctx := ctxAuth(authz.ScopeAll, nil, "u1")

	repo.items["a1"] = &entity.Attachment{
		ID: "a1", TenantID: "t1", ConversationID: "cv1",
		ContentType: "audio/ogg", Filename: "a.ogg", StorageKey: "k/a.ogg", Size: 10, Status: entity.StatusPending,
	}
	att, err := svc.Confirm(ctx, contracts.ConfirmUpload{AttachmentID: "a1"})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if conviter.called != 0 {
		t.Errorf("non-webm audio must not be converted")
	}
	if att.ContentType != "audio/ogg" || att.Filename != "a.ogg" || att.StorageKey != "k/a.ogg" {
		t.Errorf("non-webm attachment must be untouched: %+v", att)
	}
}

// (c) A conversion failure preserves the original upload and never fails confirm.
func TestConfirm_RemuxFailureKeepsOriginal(t *testing.T) {
	repo := newRepo()
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	st := &fakeStorage{provider: "s3", getData: []byte("webm")}
	svc := newSvc(repo, conv, &fakeMsgRepo{}, st, Config{DownloadBaseURL: "http://api"})
	svc.SetAudioConverter(&fakeAudioConverter{err: errInjected})
	ctx := ctxAuth(authz.ScopeAll, nil, "u1")

	repo.items["a1"] = &entity.Attachment{
		ID: "a1", TenantID: "t1", ConversationID: "cv1",
		ContentType: "audio/webm", Filename: "voice.webm", StorageKey: "k/voice.webm", Size: 5, Status: entity.StatusPending,
	}
	att, err := svc.Confirm(ctx, contracts.ConfirmUpload{AttachmentID: "a1"})
	if err != nil {
		t.Fatalf("confirm must not fail when remux fails: %v", err)
	}
	if att.Status != entity.StatusReady {
		t.Errorf("attachment must still be ready")
	}
	if att.ContentType != "audio/webm" || att.Filename != "voice.webm" || att.StorageKey != "k/voice.webm" {
		t.Errorf("a failed remux must keep the original: %+v", att)
	}
	if len(st.deleted) != 0 {
		t.Errorf("a failed remux must not delete the original")
	}
}

var errInjected = errors.New("ffmpeg boom")

// IntegrationMediaURL (the URL delivered to external systems via the outbound
// webhook) appends the content-type extension just like the internal mediaURL — so
// the gateway/Nexxa can infer the type from the URL. This is the URL that reaches
// WhatsApp, so .ogg here is what fixes audio.
func TestIntegrationMediaURL_AppendsExtension(t *testing.T) {
	repo := newRepo()
	svc := newSvc(repo, &fakeConvRepo{}, &fakeMsgRepo{}, &fakeStorage{provider: "s3"},
		Config{DownloadBaseURL: "http://api", SigningSecret: "s3cr3t", MediaURLTTL: time.Minute})
	ctx := ctxAuth(authz.ScopeAll, nil, "u1")

	cases := []struct{ id, ct, suffix string }{
		{"aud", "audio/ogg", ".ogg"},
		{"img", "image/jpeg", ".jpg"},
		{"unknown", "application/zip", ""},
	}
	for _, c := range cases {
		repo.items[c.id] = &entity.Attachment{
			ID: c.id, TenantID: "t1", StorageKey: "k/" + c.id, ContentType: c.ct,
			Filename: "f", Status: entity.StatusReady,
		}
		url, err := svc.IntegrationMediaURL(ctx, c.id)
		if err != nil {
			t.Fatalf("%s: %v", c.id, err)
		}
		if !strings.HasPrefix(url, "http://api/v1/channel-media/") {
			t.Errorf("%s: wrong prefix: %q", c.id, url)
		}
		if c.suffix == "" {
			for _, ext := range knownMediaExtensions {
				if strings.HasSuffix(url, ext) {
					t.Errorf("%s (unknown type) must have no extension: %q", c.id, url)
				}
			}
		} else if !strings.HasSuffix(url, c.suffix) {
			t.Errorf("%s must end with %q: %q", c.id, c.suffix, url)
		}
	}
}

// After a remux Confirm, the PERSISTED record (re-read via FindByID) must reflect the
// .ogg result — content_type, filename and storage_key — and IntegrationMediaURL,
// which re-reads that record, must then produce a .ogg URL for the gateway. Guards
// the bug where only signed_url was persisted while content_type/storage_key stayed
// webm, so the integration URL came out extension-less.
func TestConfirm_RemuxPersistsRecordForIntegrationURL(t *testing.T) {
	repo := newRepo()
	conv := &fakeConvRepo{conv: &conventity.Conversation{ID: "cv1", TenantID: "t1", SectorID: "s1"}}
	st := &fakeStorage{provider: "s3", getData: []byte("webm")}
	svc := newSvc(repo, conv, &fakeMsgRepo{}, st,
		Config{DownloadBaseURL: "http://api", SigningSecret: "s3cr3t", MediaURLTTL: time.Minute})
	svc.SetAudioConverter(&fakeAudioConverter{out: []byte("OGG")})
	ctx := ctxAuth(authz.ScopeAll, nil, "u1")

	repo.items["a1"] = &entity.Attachment{
		ID: "a1", TenantID: "t1", ConversationID: "cv1",
		ContentType: "audio/webm;codecs=opus", Filename: "voice.webm",
		StorageKey: "attachments/t1/cv1/a1/voice.webm", Size: 999, Status: entity.StatusPending,
	}
	if _, err := svc.Confirm(ctx, contracts.ConfirmUpload{AttachmentID: "a1"}); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	got, err := repo.FindByID(ctx, "a1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.ContentType != "audio/ogg" {
		t.Errorf("persisted content_type = %q, want audio/ogg", got.ContentType)
	}
	if got.Filename != "voice.ogg" {
		t.Errorf("persisted filename = %q, want voice.ogg", got.Filename)
	}
	if got.StorageKey != "attachments/t1/cv1/a1/voice.ogg" {
		t.Errorf("persisted storage_key = %q, want .../voice.ogg", got.StorageKey)
	}

	url, err := svc.IntegrationMediaURL(ctx, "a1")
	if err != nil {
		t.Fatalf("integration url: %v", err)
	}
	if !strings.HasSuffix(url, ".ogg") {
		t.Errorf("integration URL (delivered to the gateway) must end with .ogg, got %q", url)
	}
}
