package factories

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	aservice "github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/service"
	arservice "github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/service"
	ctservice "github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/service"
	iamservice "github.com/romerito007/chat-smsnet-omnichannel/domain/iam/service"
	sectorservice "github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/service"
	infraautomationrules "github.com/romerito007/chat-smsnet-omnichannel/infra/automationrules"
	arrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/automationrules"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/contacts"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/conversations"
	webhookrepo "github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/repositories/webhooks"
	arctl "github.com/romerito007/chat-smsnet-omnichannel/presenter/controller/automationrules"
)

// AutomationRuleService builds the automation-rules service (CRUD + log reads). It
// validates referenced webhooks against the webhooks subscription repository and
// agent/sector/tag/attachment references via the RefChecker (also the rule health
// indicator).
func AutomationRuleService(c *container.Container) *arservice.RuleService {
	svc := arservice.NewRuleService(
		arrepo.NewRuleRepository(c.Mongo.DB),
		webhookrepo.NewSubscriptionRepository(c.Mongo.DB, c.Cipher),
		arrepo.NewLogRepository(c.Mongo.DB),
		clock,
	)
	svc.SetRefChecker(ruleRefChecker{
		users:       UserService(c),
		sectors:     SectorService(c),
		tags:        ConversationToolsTagService(c),
		attachments: AttachmentService(c),
	})
	return svc
}

// ruleRefChecker resolves an action's referenced entity existence for rule
// validation + the health indicator. A not_found maps to "does not exist"; any
// other backend error is surfaced (callers treat it as "exists", best-effort).
type ruleRefChecker struct {
	users       *iamservice.UserService
	sectors     *sectorservice.Service
	tags        *ctservice.TagService
	attachments *aservice.Service
}

// Exists implements arservice.RefChecker.
func (rc ruleRefChecker) Exists(ctx context.Context, kind, id string) (bool, error) {
	switch kind {
	case "agent":
		_, err := rc.users.Get(ctx, id)
		return existsFromErr(err)
	case "sector":
		_, err := rc.sectors.Get(ctx, id)
		return existsFromErr(err)
	case "tag":
		err := rc.tags.ValidateTags(ctx, []string{id})
		if err == nil {
			return true, nil
		}
		if code := apperror.From(err).Code; code == apperror.CodeNotFound || code == apperror.CodeValidation {
			return false, nil
		}
		return false, err
	case "attachment":
		return rc.attachments.AttachmentReady(ctx, id)
	default:
		return true, nil
	}
}

// existsFromErr maps a FindByID-style result to an existence bool: nil → exists,
// not_found → absent, any other error is surfaced.
func existsFromErr(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	if apperror.From(err).Code == apperror.CodeNotFound {
		return false, nil
	}
	return false, err
}

// AutomationRuleSink builds the event sink (Asynq enqueuer) that the conversation
// service calls to evaluate rules off the hot path.
func AutomationRuleSink(c *container.Container) *infraautomationrules.Enqueuer {
	return infraautomationrules.NewEnqueuer(c.AsynqClient)
}

// AutomationRuleEvaluator builds the async evaluator (the automationrule.evaluate
// worker handler). It reuses the webhooks dispatcher (EmitTo) for delivery and a
// Redis deduper for the anti-loop guard.
func AutomationRuleEvaluator(c *container.Container) *arservice.Evaluator {
	// The executor runs the rule's actions: send_webhook via the webhooks
	// dispatcher; send_message/send_attachment + every state mutation through the
	// conversations service (reusing the normal pipelines, under origin=automation).
	// The budget limiter is the layer-2 safety fuse for message-creating actions.
	executor := arservice.NewExecutor(
		WebhookDispatcher(c),
		conversationServiceBase(c),
		infraautomationrules.NewBudget(c.Redis),
	)
	return arservice.NewEvaluator(
		arrepo.NewRuleRepository(c.Mongo.DB),
		arrepo.NewLogRepository(c.Mongo.DB),
		convrepo.NewConversationRepository(c.Mongo.DB),
		contactrepo.New(c.Mongo.DB),
		executor,
		infraautomationrules.NewDeduper(c.Redis),
		c.Locker,
		clock,
	)
}

// AutomationRuleController builds the automation-rules CRUD + logs controller.
func AutomationRuleController(c *container.Container) *arctl.Controller {
	return arctl.NewController(AutomationRuleService(c))
}
