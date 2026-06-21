package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	auditservice "github.com/romerito007/chat-smsnet-omnichannel/domain/audit/service"
	privcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/contracts"
	privservice "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/service"
	auditrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/audit"
	privrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/privacy"
	infraprivacy "github.com/romerito007/chat-smsnet-omnichannel/infra/privacy"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/storage"
	auditctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/audit"
	privctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/privacy"
)

// AuditService builds the audit writer/reader. It implements shared.Auditor and
// is the single audit trail consulted by the privacy domain ("toda ação
// auditada").
func AuditService(c *container.Container) *auditservice.Service {
	return auditservice.NewService(auditrepo.New(c.Mongo.DB), clock)
}

// AuditController builds the audit-log controller, wired with the agent (IAM)
// directory so actor_id resolves to actor_name (raw id kept).
func AuditController(c *container.Container) *auditctl.Controller {
	return auditctl.NewController(AuditService(c)).SetDirectories(UserService(c))
}

// PrivacyFileStore builds the export file store (local filesystem + signed URLs).
func PrivacyFileStore(c *container.Container) privcontracts.FileStore {
	return storage.NewLocalFileStore(
		c.Config.Privacy.StorageDir,
		c.Config.Privacy.SigningSecret,
		c.Config.Privacy.DownloadBaseURL,
	)
}

// PrivacyEnqueuer builds the privacy.export Asynq enqueuer.
func PrivacyEnqueuer(c *container.Container) *infraprivacy.Enqueuer {
	return infraprivacy.NewEnqueuer(c.AsynqClient)
}

// PrivacyService builds the privacy (LGPD) service. The configured attachments
// storage backend is passed as the BlobStore so contact erasure can purge media
// blobs alongside the database rows.
func PrivacyService(c *container.Container) *privservice.Service {
	blobs, _ := attachmentStorage(c)
	return privservice.NewService(
		privrepo.New(c.Mongo.DB),
		PrivacyFileStore(c),
		blobs,
		PrivacyEnqueuer(c),
		AuditService(c),
		clock,
		c.Config.Privacy.DownloadTTL,
	)
}

// PrivacyController builds the privacy controller.
func PrivacyController(c *container.Container) *privctl.Controller {
	return privctl.NewController(PrivacyService(c), PrivacyFileStore(c))
}
