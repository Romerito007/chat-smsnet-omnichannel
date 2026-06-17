package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	groupservice "github.com/romerito007/chat-smsnet-omnichannel/domain/groups/service"
	grouprepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/groups"
	groupctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/groups"
)

// GroupService builds the WhatsApp groups service, wired to the webhook dispatcher
// (the channel-managed webhook is how a sync request reaches the gateway) and the
// audit trail.
func GroupService(c *container.Container) *groupservice.Service {
	svc := groupservice.New(
		grouprepo.New(c.Mongo.DB, clock),
		WebhookDispatcher(c),
		clock,
	)
	svc.SetAuditor(AuditService(c))
	return svc
}

// GroupController builds the groups management controller.
func GroupController(c *container.Container) *groupctl.Controller {
	return groupctl.NewController(GroupService(c))
}
