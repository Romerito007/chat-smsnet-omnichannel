package start_routines

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/app/factories"
)

// corsConfigurer is the optional capability a storage backend exposes when it can
// apply a browser-upload CORS policy to itself (only the S3 backend does).
type corsConfigurer interface {
	EnsureCORS(ctx context.Context, origins []string) error
}

// bootstrapAttachmentsCORS self-heals the S3 bucket CORS on boot so the SPA can
// PUT/GET objects directly via presigned URLs (otherwise the cross-origin
// preflight is rejected by S3 and no direct upload works). Best-effort: a missing
// s3:PutBucketCORS permission or a non-S3 backend only logs — never blocks
// startup, since the policy can also be applied manually (see deploy/s3-cors.json).
func bootstrapAttachmentsCORS(ctx context.Context, c *container.Container) {
	s3cfg := c.Config.Attachments.S3
	if c.Config.Attachments.Provider != "s3" || s3cfg.Bucket == "" || !s3cfg.EnsureCORS {
		return
	}
	store, _ := factories.AttachmentStorageBackend(c)
	cc, ok := store.(corsConfigurer)
	if !ok {
		return // backend fell back to local (e.g. invalid S3 config)
	}
	if err := cc.EnsureCORS(ctx, s3cfg.CORSAllowedOrigins); err != nil {
		c.Logger.Warn("attachments: could not apply S3 bucket CORS (apply deploy/s3-cors.json manually)",
			"bucket", s3cfg.Bucket, "error", err.Error())
		return
	}
	c.Logger.Info("attachments: S3 bucket CORS ensured", "bucket", s3cfg.Bucket, "origins", s3cfg.CORSAllowedOrigins)
}
