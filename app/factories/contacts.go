package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	contactctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/contacts"
)

// ContactController builds the contact read controller.
func ContactController(c *container.Container) *contactctl.Controller {
	return contactctl.NewController(ContactService(c))
}
