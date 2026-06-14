package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	platformservice "github.com/romerito007/chat-smsnet-omnichannel/domain/platform/service"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/iam"
	tenantrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/tenant"
	platformctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/platform"
)

// PlatformService builds the platform-plane provisioning service.
func PlatformService(c *container.Container) *platformservice.Service {
	svc := platformservice.New(
		tenantrepo.New(c.Mongo.DB),
		iamrepo.NewUserRepository(c.Mongo.DB),
		RoleService(c),
		c.Hasher,
		c.Tokens,
		clock,
	)
	svc.SetAuditor(AuditService(c))
	return svc
}

// PlatformController builds the platform provisioning controller.
func PlatformController(c *container.Container) *platformctl.Controller {
	return platformctl.NewController(PlatformService(c))
}
