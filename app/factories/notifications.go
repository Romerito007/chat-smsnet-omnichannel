package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	nservice "github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/service"
	iamrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/iam"
	notifrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/notifications"
	infraemail "github.com/romerito007/chat-smsnet-omnichannel/infra/email"
	infranotif "github.com/romerito007/chat-smsnet-omnichannel/infra/notifications"
	notifctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/notifications"
)

// NotificationEnqueuer builds the Asynq enqueuer (the shared.Notifier consulted
// by producers and the email enqueuer).
func NotificationEnqueuer(c *container.Container) *infranotif.Enqueuer {
	return infranotif.NewEnqueuer(c.AsynqClient)
}

// NotificationService builds the notifications service (the notification.send /
// notification.email handlers plus the user inbox and preferences).
func NotificationService(c *container.Container) *nservice.Service {
	return nservice.NewService(
		notifrepo.NewNotificationRepository(c.Mongo.DB),
		notifrepo.NewPreferencesRepository(c.Mongo.DB),
		iamrepo.NewUserRepository(c.Mongo.DB),
		c.Events,
		NotificationEnqueuer(c),
		infraemail.NewSender(c.Logger, c.Config.Notifications.EmailFrom),
		clock,
	)
}

// NotificationController builds the notifications controller.
func NotificationController(c *container.Container) *notifctl.Controller {
	return notifctl.NewController(NotificationService(c))
}
