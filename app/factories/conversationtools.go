package factories

import (
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	ctservice "github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/service"
	ctrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversationtools"
	ctctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/conversationtools"
)

// ConversationToolsTagService builds the tag service (also the conversations
// TagCatalog).
func ConversationToolsTagService(c *container.Container) *ctservice.TagService {
	return ctservice.NewTagService(ctrepo.NewTagRepository(c.Mongo.DB), clock)
}

// ConversationToolsCannedService builds the canned-response service.
func ConversationToolsCannedService(c *container.Container) *ctservice.CannedResponseService {
	return ctservice.NewCannedResponseService(ctrepo.NewCannedResponseRepository(c.Mongo.DB), clock)
}

// ConversationToolsCloseReasonService builds the close-reason service (also the
// conversations CloseReasonPolicy).
func ConversationToolsCloseReasonService(c *container.Container) *ctservice.CloseReasonService {
	return ctservice.NewCloseReasonService(ctrepo.NewCloseReasonRepository(c.Mongo.DB), clock)
}

// ConversationToolsController builds the conversationtools controller, wiring the
// conversations service (with the tag catalog + close-reason policy already
// attached) for the tag-apply endpoint.
func ConversationToolsController(c *container.Container) *ctctl.Controller {
	return ctctl.NewController(
		ConversationToolsTagService(c),
		ConversationToolsCannedService(c),
		ConversationToolsCloseReasonService(c),
		ConversationService(c),
	)
}
