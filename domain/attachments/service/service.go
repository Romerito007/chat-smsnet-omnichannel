// Package service holds the attachments business logic: issuing signed upload
// URLs, confirming uploads and linking them to messages, and resolving downloads
// behind a conversation-access check.
package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// Config bounds uploads and the signed-URL lifetimes.
type Config struct {
	MaxSizeBytes        int64
	AvatarMaxSizeBytes  int64    // caps avatar uploads (image/* only)
	AllowedContentTypes []string // empty = allow any
	UploadTTL           time.Duration
	DownloadTTL         time.Duration
	// DownloadBaseURL is the public API origin used to build the stable, access-
	// gated download URL stored on the record.
	DownloadBaseURL string
	// SigningSecret signs the public, JWT-less channel-media URLs used on the
	// INTEGRATION rail (outbound delivery to an external system).
	SigningSecret string
	// MediaURLTTL bounds the signed channel-media URL lifetime.
	MediaURLTTL time.Duration
	// AvatarURLTTL bounds the short-lived signed avatar URL resolved into
	// Contact/User payloads. Defaults to 15m.
	AvatarURLTTL time.Duration
}

// Service implements the attachments use cases.
type Service struct {
	repo          repository.Repository
	storage       contracts.Storage
	conversations convrepo.ConversationRepository
	messages      convrepo.MessageRepository
	clock         shared.Clock
	cfg           Config
}

// NewService builds the service applying sane defaults.
func NewService(repo repository.Repository, storage contracts.Storage, conversations convrepo.ConversationRepository, messages convrepo.MessageRepository, clock shared.Clock, cfg Config) *Service {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if cfg.MaxSizeBytes <= 0 {
		cfg.MaxSizeBytes = 25 << 20 // 25 MiB
	}
	if cfg.AvatarMaxSizeBytes <= 0 {
		cfg.AvatarMaxSizeBytes = 5 << 20 // 5 MiB
	}
	if cfg.UploadTTL <= 0 {
		cfg.UploadTTL = 15 * time.Minute
	}
	if cfg.DownloadTTL <= 0 {
		cfg.DownloadTTL = 5 * time.Minute
	}
	if cfg.MediaURLTTL <= 0 {
		cfg.MediaURLTTL = 24 * time.Hour
	}
	if cfg.AvatarURLTTL <= 0 {
		cfg.AvatarURLTTL = 15 * time.Minute
	}
	return &Service{repo: repo, storage: storage, conversations: conversations, messages: messages, clock: clock, cfg: cfg}
}

// ── integration rail: signed, JWT-less channel-media URL ───────────────────────

// IntegrationMediaURL builds a signed, public channel-media URL for an attachment,
// for delivery to an EXTERNAL system on the integration rail (the internal
// download URL is JWT-gated and unusable by an integrator). The token encodes the
// storage key + content-type + filename + expiry, HMAC-signed, so the public
// handler serves the object without a JWT or DB lookup. Tenant-scoped lookup.
func (s *Service) IntegrationMediaURL(ctx context.Context, attachmentID string) (string, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return "", err
	}
	att, err := s.repo.FindByID(ctx, strings.TrimSpace(attachmentID))
	if err != nil {
		return "", err
	}
	if att.Status != entity.StatusReady {
		return "", apperror.Conflict("attachment upload not confirmed")
	}
	exp := s.clock.Now().Add(s.cfg.MediaURLTTL).UnixMilli()
	token := s.signMediaToken(att.StorageKey, att.ContentType, att.Filename, exp)
	return strings.TrimRight(s.cfg.DownloadBaseURL, "/") + "/v1/channel-media/" + token, nil
}

// SignedAvatarURLs batch-resolves avatar attachment ids to short-lived, JWT-less
// media URLs (the same signed /v1/channel-media/{token} mechanism), so a list
// page renders avatars directly in <img src> without a per-item Authorization
// request. Only ready, same-tenant image attachments get a URL; others are absent
// from the map. One FindByIDs query; token signing is pure local HMAC (no per-item
// IO), so this stays cheap for large pages.
func (s *Service) SignedAvatarURLs(ctx context.Context, ids []string) (map[string]string, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	cleaned := dedupeNonEmpty(ids)
	if len(cleaned) == 0 {
		return nil, nil
	}
	found, err := s.repo.FindByIDs(ctx, cleaned)
	if err != nil {
		return nil, err
	}
	exp := s.clock.Now().Add(s.cfg.AvatarURLTTL).UnixMilli()
	out := make(map[string]string, len(found))
	for _, att := range found {
		if att.Status != entity.StatusReady || !isImageContentType(att.ContentType) {
			continue
		}
		token := s.signMediaToken(att.StorageKey, att.ContentType, att.Filename, exp)
		out[att.ID] = strings.TrimRight(s.cfg.DownloadBaseURL, "/") + "/v1/channel-media/" + token
	}
	return out, nil
}

// DownloadSigned resolves a signed channel-media token (no JWT, no tenant): it
// verifies the signature + expiry and serves the object from storage.
func (s *Service) DownloadSigned(token string) (contracts.DownloadResult, error) {
	key, contentType, filename, err := s.verifyMediaToken(token)
	if err != nil {
		return contracts.DownloadResult{}, err
	}
	res, err := s.storage.Download(key, filename, s.cfg.DownloadTTL)
	if err != nil {
		return contracts.DownloadResult{}, err
	}
	if res.ContentType == "" {
		res.ContentType = contentType
	}
	if res.Filename == "" {
		res.Filename = filename
	}
	return res, nil
}

func (s *Service) signMediaToken(key, contentType, filename string, expMillis int64) string {
	payload := fmt.Sprintf("%d|%s|%s|%s", expMillis,
		base64.RawURLEncoding.EncodeToString([]byte(contentType)),
		base64.RawURLEncoding.EncodeToString([]byte(filename)),
		base64.RawURLEncoding.EncodeToString([]byte(key)))
	b64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return b64 + "." + s.hmacHex(b64)
}

func (s *Service) verifyMediaToken(token string) (key, contentType, filename string, err error) {
	bad := apperror.Forbidden("invalid or expired media token")
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 || !hmac.Equal([]byte(parts[1]), []byte(s.hmacHex(parts[0]))) {
		return "", "", "", bad
	}
	raw, derr := base64.RawURLEncoding.DecodeString(parts[0])
	if derr != nil {
		return "", "", "", bad
	}
	f := strings.SplitN(string(raw), "|", 4)
	if len(f) != 4 {
		return "", "", "", bad
	}
	expMillis, perr := strconv.ParseInt(f[0], 10, 64)
	if perr != nil || s.clock.Now().After(time.UnixMilli(expMillis)) {
		return "", "", "", bad
	}
	ct, _ := base64.RawURLEncoding.DecodeString(f[1])
	fn, _ := base64.RawURLEncoding.DecodeString(f[2])
	k, kerr := base64.RawURLEncoding.DecodeString(f[3])
	if kerr != nil {
		return "", "", "", bad
	}
	return string(k), string(ct), string(fn), nil
}

func (s *Service) hmacHex(payload string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.SigningSecret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// RequestUploadURL validates the request, reserves a pending attachment and
// returns a signed target for the client to upload the bytes directly to storage.
func (s *Service) RequestUploadURL(ctx context.Context, cmd contracts.CreateUploadURL) (*entity.Attachment, contracts.UploadTarget, error) {
	if cmd.Avatar != nil {
		return s.requestAvatarUploadURL(ctx, cmd)
	}

	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, contracts.UploadTarget{}, err
	}
	conv, err := s.loadVisible(ctx, strings.TrimSpace(cmd.ConversationID))
	if err != nil {
		return nil, contracts.UploadTarget{}, err
	}

	filename := sanitizeFilename(cmd.Filename)
	contentType := strings.TrimSpace(cmd.ContentType)
	if v := s.validate(filename, contentType, cmd.Size); v != nil {
		return nil, contracts.UploadTarget{}, v
	}

	id := shared.NewID()
	key := fmt.Sprintf("attachments/%s/%s/%s/%s", tenantID, conv.ID, id, filename)
	return s.reserveUpload(ctx, id, tenantID, conv.ID, key, filename, contentType, cmd.Size)
}

// requestAvatarUploadURL issues a conversation-less upload for an avatar: no
// visibility check (tenant scope only), key namespaced under avatars/, content
// type restricted to image/* and bounded by the avatar size limit.
func (s *Service) requestAvatarUploadURL(ctx context.Context, cmd contracts.CreateUploadURL) (*entity.Attachment, contracts.UploadTarget, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, contracts.UploadTarget{}, err
	}
	ownerType := strings.ToLower(strings.TrimSpace(cmd.Avatar.OwnerType))
	ownerID := strings.TrimSpace(cmd.Avatar.OwnerID)
	if ownerType != "contacts" && ownerType != "users" {
		return nil, contracts.UploadTarget{}, apperror.Validation("invalid avatar owner").
			WithDetails(map[string]any{"avatar.owner_type": "must be contacts or users"})
	}
	if ownerID == "" {
		return nil, contracts.UploadTarget{}, apperror.Validation("invalid avatar owner").
			WithDetails(map[string]any{"avatar.owner_id": "is required"})
	}

	filename := sanitizeFilename(cmd.Filename)
	contentType := strings.TrimSpace(cmd.ContentType)
	if v := s.validateAvatar(filename, contentType, cmd.Size); v != nil {
		return nil, contracts.UploadTarget{}, v
	}

	id := shared.NewID()
	key := fmt.Sprintf("avatars/%s/%s/%s/%s", tenantID, ownerType, ownerID, filename)
	return s.reserveUpload(ctx, id, tenantID, "", key, filename, contentType, cmd.Size)
}

// reserveUpload signs the storage target and persists the pending record shared
// by the conversation and avatar upload paths. conversationID is "" for avatars.
func (s *Service) reserveUpload(ctx context.Context, id, tenantID, conversationID, key, filename, contentType string, size int64) (*entity.Attachment, contracts.UploadTarget, error) {
	target, err := s.storage.SignUpload(key, contentType, size, s.cfg.UploadTTL)
	if err != nil {
		return nil, contracts.UploadTarget{}, apperror.Internal("could not sign upload url").Wrap(err)
	}
	att := &entity.Attachment{
		ID:              id,
		TenantID:        tenantID,
		ConversationID:  conversationID,
		Filename:        filename,
		ContentType:     contentType,
		Size:            size,
		StorageProvider: s.storage.Provider(),
		StorageKey:      key,
		Status:          entity.StatusPending,
		CreatedBy:       actorID(ctx),
		CreatedAt:       s.clock.Now(),
	}
	if err := s.repo.Create(ctx, att); err != nil {
		return nil, contracts.UploadTarget{}, err
	}
	return att, target, nil
}

// Confirm marks a pending attachment ready and optionally links it to a message,
// returning the record with its stable download URL.
func (s *Service) Confirm(ctx context.Context, cmd contracts.ConfirmUpload) (*entity.Attachment, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	att, err := s.repo.FindByID(ctx, strings.TrimSpace(cmd.AttachmentID))
	if err != nil {
		return nil, err
	}
	// Enforce access on confirm too (avatars: tenant scope only).
	if err := s.authorizeAttachment(ctx, att); err != nil {
		return nil, err
	}

	if mid := strings.TrimSpace(cmd.MessageID); mid != "" {
		msg, err := s.messages.FindByID(ctx, mid)
		if err != nil {
			return nil, apperror.Validation("message not found")
		}
		if msg.ConversationID != att.ConversationID {
			return nil, apperror.Validation("message does not belong to the attachment's conversation")
		}
		att.MessageID = mid
	}

	// The object must actually have been uploaded before we mark it ready.
	exists, err := s.storage.Exists(att.StorageKey)
	if err != nil {
		return nil, apperror.Internal("could not verify the upload").Wrap(err)
	}
	if !exists {
		return nil, apperror.Validation("the file was not uploaded to storage")
	}

	// Metadata (content_type/filename/size) is captured at upload-url time from the
	// client and validated there (non-empty content_type, size > 0). Guard against
	// a record reaching "ready" with no content_type — a downstream message
	// hydration must never yield an empty content_type. Default rather than fail.
	if strings.TrimSpace(att.ContentType) == "" {
		att.ContentType = "application/octet-stream"
	}
	att.Status = entity.StatusReady
	att.SignedURL = s.downloadURL(att.ID)
	if err := s.repo.Update(ctx, att); err != nil {
		return nil, err
	}
	return att, nil
}

// StoreInbound persists a raw inbound attachment (Chatwoot multipart) directly to
// storage and creates a ready record linked to the conversation, returning the
// conversation-entity attachment with its access-gated download URL. It is a
// system path (inbound is pre-auth: the channel token already authenticated), so
// it does NOT run the agent visibility check — the tenant comes from the context.
func (s *Service) StoreInbound(ctx context.Context, conversationID, filename, contentType string, data []byte) (conventity.Attachment, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return conventity.Attachment{}, err
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return conventity.Attachment{}, apperror.Validation("conversation_id is required")
	}
	filename = sanitizeFilename(filename)
	contentType = strings.TrimSpace(contentType)
	size := int64(len(data))
	if v := s.validate(filename, contentType, size); v != nil {
		return conventity.Attachment{}, v
	}

	id := shared.NewID()
	key := fmt.Sprintf("attachments/%s/%s/%s/%s", tenantID, conversationID, id, filename)
	if err := s.storage.Put(key, contentType, data); err != nil {
		return conventity.Attachment{}, apperror.Internal("could not store inbound attachment").Wrap(err)
	}

	att := &entity.Attachment{
		ID:              id,
		TenantID:        tenantID,
		ConversationID:  conversationID,
		Filename:        filename,
		ContentType:     contentType,
		Size:            size,
		StorageProvider: s.storage.Provider(),
		StorageKey:      key,
		Status:          entity.StatusReady,
		SignedURL:       s.downloadURL(id),
		CreatedAt:       s.clock.Now(),
	}
	if err := s.repo.Create(ctx, att); err != nil {
		return conventity.Attachment{}, err
	}
	return conventity.Attachment{
		ID: att.ID, URL: att.SignedURL, ContentType: contentType, Filename: filename, Size: size,
	}, nil
}

// Download resolves an attachment for serving after checking the caller's access
// to its conversation. The raw object is never served without this check.
func (s *Service) Download(ctx context.Context, id string) (contracts.DownloadResult, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return contracts.DownloadResult{}, err
	}
	att, err := s.repo.FindByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return contracts.DownloadResult{}, err
	}
	if err := s.authorizeAttachment(ctx, att); err != nil {
		return contracts.DownloadResult{}, err
	}
	if att.Status != entity.StatusReady {
		return contracts.DownloadResult{}, apperror.Conflict("attachment upload not confirmed")
	}
	res, err := s.storage.Download(att.StorageKey, att.Filename, s.cfg.DownloadTTL)
	if err != nil {
		return contracts.DownloadResult{}, err
	}
	// The local backend serves raw bytes without metadata; fill from the record.
	if res.ContentType == "" {
		res.ContentType = att.ContentType
	}
	if res.Filename == "" {
		res.Filename = att.Filename
	}
	return res, nil
}

// Get returns an attachment's metadata (access-checked).
func (s *Service) Get(ctx context.Context, id string) (*entity.Attachment, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	att, err := s.repo.FindByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if err := s.authorizeAttachment(ctx, att); err != nil {
		return nil, err
	}
	return att, nil
}

func (s *Service) validate(filename, contentType string, size int64) error {
	v := map[string]any{}
	if filename == "" {
		v["filename"] = "is required"
	}
	if contentType == "" {
		v["content_type"] = "is required"
	} else if !s.contentTypeAllowed(contentType) {
		v["content_type"] = "is not allowed"
	}
	if size <= 0 {
		v["size"] = "must be greater than zero"
	} else if size > s.cfg.MaxSizeBytes {
		v["size"] = fmt.Sprintf("exceeds the %d byte limit", s.cfg.MaxSizeBytes)
	}
	if len(v) > 0 {
		return apperror.Validation("invalid attachment").WithDetails(v)
	}
	return nil
}

// validateAvatar enforces the avatar policy: a filename, an image/* content type
// (regardless of the general allow-list) and a size within the avatar limit.
func (s *Service) validateAvatar(filename, contentType string, size int64) error {
	v := map[string]any{}
	if filename == "" {
		v["filename"] = "is required"
	}
	if contentType == "" {
		v["content_type"] = "is required"
	} else if !isImageContentType(contentType) {
		v["content_type"] = "must be an image"
	}
	if size <= 0 {
		v["size"] = "must be greater than zero"
	} else if size > s.cfg.AvatarMaxSizeBytes {
		v["size"] = fmt.Sprintf("exceeds the %d byte avatar limit", s.cfg.AvatarMaxSizeBytes)
	}
	if len(v) > 0 {
		return apperror.Validation("invalid avatar").WithDetails(v)
	}
	return nil
}

// ValidateReadyImage verifies an attachment is a tenant-scoped, ready image — the
// check another domain (e.g. contacts) runs before persisting it as an avatar.
// An empty id is allowed (clears the avatar). Returns a 400 validation_error when
// the attachment is missing, not ready, or not an image.
func (s *Service) ValidateReadyImage(ctx context.Context, attachmentID string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	id := strings.TrimSpace(attachmentID)
	if id == "" {
		return nil
	}
	att, err := s.repo.FindByID(ctx, id) // tenant-scoped: cross-tenant ids are not found
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return apperror.Validation("avatar attachment not found").
				WithDetails(map[string]any{"avatar_attachment_id": "not found"})
		}
		return err
	}
	if att.Status != entity.StatusReady {
		return apperror.Validation("avatar attachment is not ready").
			WithDetails(map[string]any{"avatar_attachment_id": "is not ready"})
	}
	if !isImageContentType(att.ContentType) {
		return apperror.Validation("avatar attachment must be an image").
			WithDetails(map[string]any{"avatar_attachment_id": "must be an image"})
	}
	return nil
}

// isImageContentType reports whether ct is an image/* MIME type.
func isImageContentType(ct string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(ct)), "image/")
}

// HydrateAttachments batch-resolves attachment ids to their full media metadata
// (id, url=JWT-gated download, content_type, filename, size), keyed by id, for
// the conversations read boundary. Missing ids are simply absent from the map.
// Implements the conversations AttachmentResolver port.
func (s *Service) HydrateAttachments(ctx context.Context, ids []string) (map[string]conventity.Attachment, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	cleaned := dedupeNonEmpty(ids)
	if len(cleaned) == 0 {
		return nil, nil
	}
	found, err := s.repo.FindByIDs(ctx, cleaned)
	if err != nil {
		return nil, err
	}
	out := make(map[string]conventity.Attachment, len(found))
	for _, att := range found {
		out[att.ID] = conventity.Attachment{
			ID:          att.ID,
			URL:         s.downloadURL(att.ID),
			ContentType: att.ContentType,
			Filename:    att.Filename,
			Size:        att.Size,
		}
	}
	return out, nil
}

// ValidateMessageAttachments verifies every id exists in the tenant and is ready
// (uploaded + confirmed). Returns a 400 validation_error otherwise, so a message
// is never stored referencing an orphan or unconfirmed attachment.
func (s *Service) ValidateMessageAttachments(ctx context.Context, ids []string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	cleaned := dedupeNonEmpty(ids)
	if len(cleaned) == 0 {
		return nil
	}
	found, err := s.repo.FindByIDs(ctx, cleaned)
	if err != nil {
		return err
	}
	byID := make(map[string]*entity.Attachment, len(found))
	for _, att := range found {
		byID[att.ID] = att
	}
	for _, id := range cleaned {
		att, ok := byID[id]
		if !ok {
			return apperror.Validation("attachment not found").
				WithDetails(map[string]any{"attachments": "unknown attachment " + id})
		}
		if att.Status != entity.StatusReady {
			return apperror.Validation("attachment not ready").
				WithDetails(map[string]any{"attachments": "attachment " + id + " is not confirmed"})
		}
	}
	return nil
}

// dedupeNonEmpty trims, drops empty entries and de-duplicates the ids.
func dedupeNonEmpty(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *Service) contentTypeAllowed(ct string) bool {
	if len(s.cfg.AllowedContentTypes) == 0 {
		return true
	}
	ct = strings.ToLower(strings.TrimSpace(ct))
	for _, allowed := range s.cfg.AllowedContentTypes {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		if allowed == ct {
			return true
		}
		// Support "image/*" style prefixes.
		if strings.HasSuffix(allowed, "/*") && strings.HasPrefix(ct, strings.TrimSuffix(allowed, "*")) {
			return true
		}
	}
	return false
}

func (s *Service) downloadURL(id string) string {
	return strings.TrimRight(s.cfg.DownloadBaseURL, "/") + "/v1/attachments/" + id + "/download"
}

// authorizeAttachment enforces access to an already-loaded attachment. Avatar
// attachments carry no conversation, so they are authorized by tenant scope alone
// (the repo already filtered by tenant); conversation attachments run the full
// visibility check.
func (s *Service) authorizeAttachment(ctx context.Context, att *entity.Attachment) error {
	if att.ConversationID == "" {
		return nil // avatar: tenant-scoped repo lookup is the access boundary
	}
	_, err := s.loadVisible(ctx, att.ConversationID)
	return err
}

// loadVisible loads a conversation and enforces the actor's visibility, mirroring
// the conversations/providerhub access rules.
func (s *Service) loadVisible(ctx context.Context, conversationID string) (*conventity.Conversation, error) {
	if conversationID == "" {
		return nil, apperror.Validation("conversation_id is required")
	}
	ac, ok := authz.FromContext(ctx)
	if !ok {
		return nil, apperror.Unauthorized("authentication required")
	}
	conv, err := s.conversations.FindByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if ac.SectorScope == authz.ScopeAll {
		return conv, nil
	}
	if conv.AssignedTo != "" && conv.AssignedTo == ac.UserID {
		return conv, nil
	}
	for _, sid := range ac.SectorIDs {
		if sid != "" && sid == conv.SectorID {
			return conv, nil
		}
	}
	return nil, apperror.NotFound("conversation not found")
}

// sanitizeFilename strips any path and keeps a safe base name.
func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	base := path.Base(name)
	if base == "." || base == "/" {
		return ""
	}
	return base
}

func actorID(ctx context.Context) string {
	if ac, ok := authz.FromContext(ctx); ok && ac.UserID != "" {
		return ac.UserID
	}
	return ""
}
