package service

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	arcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/repository"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/repository"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// WebhookEmitter delivers an event to one webhook by id, reusing the webhooks
// pipeline (implemented by the webhooks Dispatcher.EmitTo).
type WebhookEmitter interface {
	EmitTo(ctx context.Context, tenantID, webhookID, event string, payload any) error
}

// Deduper is the anti-loop guard: Allow returns true the first time a key is seen
// within the window, false on a repeat (skip). Fail-open on backend errors.
type Deduper interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// Evaluator runs an automationrule.evaluate task: it loads the tenant's enabled
// rules for the event, hydrates the conversation + contact, matches conditions
// (AND), and fires each matching rule's actions (send_webhook) — with anti-loop
// dedup and a minimal evaluation log.
type Evaluator struct {
	rules         repository.RuleRepository
	logs          repository.LogRepository
	conversations convrepo.ConversationRepository
	contacts      contactrepo.ContactRepository
	emitter       WebhookEmitter
	dedup         Deduper
	clock         shared.Clock
}

// NewEvaluator builds the evaluator.
func NewEvaluator(
	rules repository.RuleRepository,
	logs repository.LogRepository,
	conversations convrepo.ConversationRepository,
	contacts contactrepo.ContactRepository,
	emitter WebhookEmitter,
	dedup Deduper,
	clock shared.Clock,
) *Evaluator {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Evaluator{rules: rules, logs: logs, conversations: conversations, contacts: contacts, emitter: emitter, dedup: dedup, clock: clock}
}

// Evaluate processes one task. The internal event is mapped to a rule event;
// unmapped events are ignored. The tenant is taken from the task and put on ctx.
func (e *Evaluator) Evaluate(ctx context.Context, task arcontracts.EvaluateTask) error {
	ruleEvent, ok := entity.RuleEventForInternal(task.Event)
	if !ok {
		return nil // not an event rules react to
	}
	ctx = shared.WithTenant(ctx, task.TenantID)

	rules, err := e.rules.ListEnabledByEvent(ctx, ruleEvent)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return nil // no rules for this event → nothing to do (cheap)
	}

	ec, err := e.hydrate(ctx, task.ConversationID)
	if err != nil {
		// A missing conversation (deleted/anonymized) is not a retryable failure.
		if apperror.From(err).Code == apperror.CodeNotFound {
			return nil
		}
		return err
	}

	for _, rule := range rules {
		if !rule.Matches(ec) {
			continue
		}
		if e.dedup != nil {
			allowed, derr := e.dedup.Allow(ctx, dedupKey(task.TenantID, rule.ID, task.ConversationID, string(ruleEvent)))
			if derr == nil && !allowed {
				e.log(ctx, task, rule.ID, ruleEvent, entity.EvalSkippedDedup, "")
				continue
			}
		}
		e.fire(ctx, task, rule, ruleEvent)
	}
	return nil
}

// fire runs a matching rule's actions, logging the outcome.
func (e *Evaluator) fire(ctx context.Context, task arcontracts.EvaluateTask, rule *entity.AutomationRule, ruleEvent entity.RuleEvent) {
	var firstErr string
	for _, a := range rule.Actions {
		if a.Type != entity.ActionSendWebhook {
			continue
		}
		// The webhook data is the original event payload; the event is the rule
		// wire name. EmitTo ignores the webhook's events[] AND scopes — the rule
		// already decided this delivery.
		if err := e.emitter.EmitTo(ctx, task.TenantID, a.WebhookID, string(ruleEvent), task.Data); err != nil && firstErr == "" {
			firstErr = summarize(err)
		}
	}
	status := entity.EvalActionEnqueued
	if firstErr != "" {
		status = entity.EvalError
	}
	e.log(ctx, task, rule.ID, ruleEvent, status, firstErr)
}

// hydrate loads the conversation and its contact into an EvalContext. Conditions
// resolve against the live conversation — for message_created too.
func (e *Evaluator) hydrate(ctx context.Context, conversationID string) (entity.EvalContext, error) {
	conv, err := e.conversations.FindByID(ctx, conversationID)
	if err != nil {
		return entity.EvalContext{}, err
	}
	ec := entity.EvalContext{
		Status:          string(conv.Status),
		Channel:         conv.Channel,
		AssignedAgentID: conv.AssignedTo,
		SectorID:        conv.SectorID,
		QueueID:         conv.QueueID,
		Priority:        string(conv.Priority),
		Tags:            conv.Tags,
	}
	if conv.ContactID != "" {
		if contact, cerr := e.contacts.FindByID(ctx, conv.ContactID); cerr == nil && contact != nil {
			ec.ContactPhone = contact.Phone
		}
	}
	return ec, nil
}

func (e *Evaluator) log(ctx context.Context, task arcontracts.EvaluateTask, ruleID string, event entity.RuleEvent, status entity.EvalStatus, errSummary string) {
	_ = e.logs.Create(ctx, &entity.RuleEvaluationLog{
		ID:             shared.NewID(),
		TenantID:       task.TenantID,
		RuleID:         ruleID,
		Event:          event,
		ConversationID: task.ConversationID,
		Status:         status,
		ErrorSummary:   errSummary,
		CreatedAt:      e.clock.Now(),
	})
}

func dedupKey(tenantID, ruleID, conversationID, event string) string {
	return "rule:fired:" + tenantID + ":" + ruleID + ":" + conversationID + ":" + event
}

func summarize(err error) string {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}
