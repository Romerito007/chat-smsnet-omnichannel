// Package service holds the attachments business logic: issuing signed upload
// URLs, confirming uploads and linking them to messages, and resolving downloads
// behind a conversation-access check.
package service

import (
	"context"
	"fmt"
	"path"
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
	AllowedContentTypes []string // empty = allow any
	UploadTTL           time.Duration
	DownloadTTL         time.Duration
	// DownloadBaseURL is the public API origin used to build the stable, access-
	// gated download URL stored on the record.
	DownloadBaseURL string
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
	if cfg.UploadTTL <= 0 {
		cfg.UploadTTL = 15 * time.Minute
	}
	if cfg.DownloadTTL <= 0 {
		cfg.DownloadTTL = 5 * time.Minute
	}
	return &Service{repo: repo, storage: storage, conversations: conversations, messages: messages, clock: clock, cfg: cfg}
}

// RequestUploadURL validates the request, reserves a pending attachment and
// returns a signed target for the client to upload the bytes directly to storage.
func (s *Service) RequestUploadURL(ctx context.Context, cmd contracts.CreateUploadURL) (*entity.Attachment, contracts.UploadTarget, error) {
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
	target, err := s.storage.SignUpload(key, contentType, cmd.Size, s.cfg.UploadTTL)
	if err != nil {
		return nil, contracts.UploadTarget{}, apperror.Internal("could not sign upload url").Wrap(err)
	}

	att := &entity.Attachment{
		ID:              id,
		TenantID:        tenantID,
		ConversationID:  conv.ID,
		Filename:        filename,
		ContentType:     contentType,
		Size:            cmd.Size,
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
	// Enforce conversation access on confirm too.
	if _, err := s.loadVisible(ctx, att.ConversationID); err != nil {
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

	att.Status = entity.StatusReady
	att.SignedURL = s.downloadURL(att.ID)
	if err := s.repo.Update(ctx, att); err != nil {
		return nil, err
	}
	return att, nil
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
	if _, err := s.loadVisible(ctx, att.ConversationID); err != nil {
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
	if _, err := s.loadVisible(ctx, att.ConversationID); err != nil {
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
