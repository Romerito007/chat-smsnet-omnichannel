package factories

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	contactservice "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/service"
	dcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/deals/contracts"
	dealservice "github.com/romerito007/chat-smsnet-omnichannel/domain/deals/service"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	dealrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/deals"
	dealctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/deals"
)

// DealService builds the sales-deal service, wired to the pipelines (for stage
// terminal lookup + default), a conversation lookup (CreateFromConversation) and a
// contact-existence guard. It uses a PLAIN pipeline service (no deal checker) to
// avoid a wiring cycle with PipelineController.
func DealService(c *container.Container) *dealservice.Service {
	svc := dealservice.New(dealrepo.New(c.Mongo.DB), PipelineService(c), clock)
	svc.SetAuditor(AuditService(c))
	svc.SetConversationLookup(conversationLookup{repo: convrepo.NewConversationRepository(c.Mongo.DB)})
	svc.SetContactChecker(contactChecker{contacts: ContactService(c)})
	return svc
}

// DealController builds the deal controller, wired with the contact/agent/pipeline
// directories so the Kanban renders names (contact_name, assigned_to_name + avatar,
// stage_name, pipeline_name), not raw ids.
func DealController(c *container.Container) *dealctl.Controller {
	return dealctl.NewController(DealService(c)).
		SetDirectories(ContactService(c), UserService(c), PipelineService(c))
}

// conversationLookup adapts the conversations repository to the deals port.
type conversationLookup struct {
	repo *convrepo.ConversationRepository
}

func (a conversationLookup) Conversation(ctx context.Context, id string) (*dcontracts.ConversationRef, error) {
	conv, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &dcontracts.ConversationRef{ContactID: conv.ContactID, SectorID: conv.SectorID, AssignedTo: conv.AssignedTo}, nil
}

// contactChecker adapts the contacts service to the deals contact-existence port.
type contactChecker struct{ contacts *contactservice.Service }

func (a contactChecker) ContactExists(ctx context.Context, id string) (bool, error) {
	if _, err := a.contacts.Get(ctx, id); err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
