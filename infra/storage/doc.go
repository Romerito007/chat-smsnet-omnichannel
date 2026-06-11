// Package storage abstracts attachment storage behind a common port with
// S3-compatible and local-filesystem backends, plus the LocalFileStore used for
// signed export downloads (privacy/reports). Attachment post-processing runs on
// the Asynq queue (attachment.process).
package storage
