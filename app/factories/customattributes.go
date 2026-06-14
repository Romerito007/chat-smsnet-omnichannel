package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	caservice "github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/service"
	carepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/customattributes"
	cactl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/customattributes"
)

// CustomAttributeService builds the custom-attribute definition service (CRUD +
// value validator).
func CustomAttributeService(c *container.Container) *caservice.Service {
	return caservice.New(carepo.NewDefinitionRepository(c.Mongo.DB), clock)
}

// CustomAttributeController builds the custom-attribute definition controller.
func CustomAttributeController(c *container.Container) *cactl.Controller {
	return cactl.NewController(CustomAttributeService(c))
}
