package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	searchservice "github.com/romerito007/chat-smsnet-omnichannel/domain/search/service"
	searchrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/search"
	searchctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/search"
)

// SearchService builds the Mongo-backed search service.
func SearchService(c *container.Container) *searchservice.Service {
	return searchservice.NewService(searchrepo.NewIndex(c.Mongo.DB))
}

// SearchController builds the search controller.
func SearchController(c *container.Container) *searchctl.Controller {
	return searchctl.NewController(SearchService(c))
}
