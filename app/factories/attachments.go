package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	acontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/contracts"
	aservice "github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/service"
	attachrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/attachments"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/storage"
	attachctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/attachments"
)

// attachmentStorage builds the configured storage backend. It falls back to the
// local backend when the S3 settings are incomplete, so a misconfiguration never
// breaks startup.
func attachmentStorage(c *container.Container) (acontracts.Storage, attachctl.LocalBlobStore) {
	cfg := c.Config.Attachments
	if cfg.Provider == "s3" && cfg.S3.Endpoint != "" && cfg.S3.Bucket != "" {
		s3, err := storage.NewS3AttachmentStorage(storage.S3Config{
			Endpoint:  cfg.S3.Endpoint,
			Region:    cfg.S3.Region,
			Bucket:    cfg.S3.Bucket,
			AccessKey: cfg.S3.AccessKey,
			SecretKey: cfg.S3.SecretKey,
		})
		if err == nil {
			return s3, nil // no local blob sink for the S3 backend
		}
		c.Logger.Error("attachments: invalid s3 config, falling back to local", "error", err)
	}
	local := storage.NewLocalAttachmentStorage(cfg.LocalDir, cfg.SigningSecret, cfg.BaseURL)
	return local, local
}

// AttachmentService builds the attachments service.
func AttachmentService(c *container.Container) *aservice.Service {
	store, _ := attachmentStorage(c)
	cfg := c.Config.Attachments
	return aservice.NewService(
		attachrepo.New(c.Mongo.DB),
		store,
		convrepo.NewConversationRepository(c.Mongo.DB),
		convrepo.NewMessageRepository(c.Mongo.DB),
		clock,
		aservice.Config{
			MaxSizeBytes:        cfg.MaxSizeBytes,
			AllowedContentTypes: cfg.AllowedContentTypes,
			UploadTTL:           cfg.UploadTTL,
			DownloadTTL:         cfg.DownloadTTL,
			DownloadBaseURL:     cfg.BaseURL,
		},
	)
}

// AttachmentController builds the attachments controller, wiring the local blob
// sink only when the local backend is active.
func AttachmentController(c *container.Container) *attachctl.Controller {
	store, blob := attachmentStorage(c)
	cfg := c.Config.Attachments
	svc := aservice.NewService(
		attachrepo.New(c.Mongo.DB),
		store,
		convrepo.NewConversationRepository(c.Mongo.DB),
		convrepo.NewMessageRepository(c.Mongo.DB),
		clock,
		aservice.Config{
			MaxSizeBytes:        cfg.MaxSizeBytes,
			AllowedContentTypes: cfg.AllowedContentTypes,
			UploadTTL:           cfg.UploadTTL,
			DownloadTTL:         cfg.DownloadTTL,
			DownloadBaseURL:     cfg.BaseURL,
		},
	)
	return attachctl.NewController(svc, blob)
}
