package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	arcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/repository"
	contactrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/repository"
	convrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// maxDepth caps the causal chain length (anti-loop layer 3, defense-in-depth).
// With origin suppression (layer 1) internal chains die at depth 1, so this only
// guards an unforeseen path that forgot to tag origin.
const maxDepth = 3

// lockTTL bounds the per-conversation action lock.
const lockTTL = 15 * time.Second

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
// (AND), and fires each matching rule's actions in priority order — with the
// anti-loop guards (origin, dedup/claim, depth), a per-conversation lock, a
// re-check against the live conversation (skipped_stale), and a per-action log.
type Evaluator struct {
	rules         repository.RuleRepository
	logs          repository.LogRepository
	conversations convrepo.ConversationRepository
	contacts      contactrepo.ContactRepository
	actions       ActionRunner
	dedup         Deduper
	locker        shared.Locker
	clock         shared.Clock
}

// NewEvaluator builds the evaluator.
func NewEvaluator(
	rules repository.RuleRepository,
	logs repository.LogRepository,
	conversations convrepo.ConversationRepository,
	contacts contactrepo.ContactRepository,
	actions ActionRunner,
	dedup Deduper,
	locker shared.Locker,
	clock shared.Clock,
) *Evaluator {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	if locker == nil {
		locker = shared.NoopLocker{}
	}
	return &Evaluator{rules: rules, logs: logs, conversations: conversations, contacts: contacts, actions: actions, dedup: dedup, locker: locker, clock: clock}
}

// Evaluate processes one task. The internal event is mapped to a rule event;
// unmapped events are ignored. The tenant is taken from the task and put on ctx.
func (e *Evaluator) Evaluate(ctx context.Context, task arcontracts.EvaluateTask) error {
	// ANTI-LOOP layer 1 (primary): events produced by an automation action are
	// tagged origin=automation and never evaluated — automation cannot feed itself.
	if task.Origin == string(shared.OriginAutomation) {
		return nil
	}
	// ANTI-LOOP layer 3 (defense-in-depth): bound the causal chain length.
	if task.Depth > maxDepth {
		return nil
	}
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

	msgContent := messageContent(task)
	ec, err := e.hydrate(ctx, task.ConversationID, msgContent)
	if err != nil {
		// A missing conversation (deleted/anonymized) is not a retryable failure.
		if apperror.From(err).Code == apperror.CodeNotFound {
			return nil
		}
		return err
	}

	// Deterministic order: priority asc, then created_at, then id.
	matched := make([]*entity.AutomationRule, 0, len(rules))
	for _, r := range rules {
		if r.Matches(ec) {
			matched = append(matched, r)
		}
	}
	if len(matched) == 0 {
		return nil
	}
	sort.SliceStable(matched, func(i, j int) bool {
		a, b := matched[i], matched[j]
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.ID < b.ID
	})

	// Serialize action execution on this conversation so concurrent events don't
	// race its state. Best-effort: a lock outage never blocks evaluation.
	release, _, lerr := e.locker.Acquire(ctx, "rule:conv:"+task.TenantID+":"+task.ConversationID, lockTTL)
	if lerr == nil {
		defer release()
	}

	// Re-hydrate under the lock: a rule that matched when emitted may no longer
	// match the LIVE conversation (skipped_stale).
	live, err := e.hydrate(ctx, task.ConversationID, msgContent)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return nil
		}
		return err
	}

	for _, rule := range matched {
		if !rule.Matches(live) {
			e.log(ctx, task, rule.ID, ruleEvent, "", entity.EvalSkippedStale, "")
			continue
		}
		// Dedup CLAIMS the (rule, event_id) firing BEFORE running actions, so an
		// Asynq retry of the same task finds the key claimed and skips — never
		// re-running a side-effectful action (e.g. send_message twice).
		if e.dedup != nil {
			allowed, derr := e.dedup.Allow(ctx, dedupKey(task.TenantID, rule.ID, task.EventID))
			if derr == nil && !allowed {
				e.log(ctx, task, rule.ID, ruleEvent, "", entity.EvalSkippedDedup, "")
				continue
			}
		}
		e.fire(ctx, task, rule, ruleEvent)
	}
	return nil
}

// fire runs a matching rule's actions in declared order, best-effort: a failing
// action does not abort the rest. Each action's outcome is logged on its own row.
func (e *Evaluator) fire(ctx context.Context, task arcontracts.EvaluateTask, rule *entity.AutomationRule, ruleEvent entity.RuleEvent) {
	ac := ActionContext{
		TenantID:       task.TenantID,
		ConversationID: task.ConversationID,
		RuleID:         rule.ID,
		EventWire:      string(ruleEvent),
		Data:           task.Data,
		Depth:          task.Depth,
	}
	for _, a := range rule.Actions {
		status, summary := classify(e.actions.Run(ctx, a, ac))
		e.log(ctx, task, rule.ID, ruleEvent, string(a.Type), status, summary)
	}
}

// classify maps an action's error to an evaluation status: budget fuse and missing
// referenced entity are soft skips; everything else is an error; nil is success.
func classify(err error) (entity.EvalStatus, string) {
	switch {
	case err == nil:
		return entity.EvalActionEnqueued, ""
	case errors.Is(err, ErrBudgetExceeded):
		return entity.EvalSkippedBudget, ""
	case apperror.From(err).Code == apperror.CodeNotFound:
		return entity.EvalSkippedMissingRef, summarize(err)
	default:
		return entity.EvalError, summarize(err)
	}
}

// messageContent extracts the triggering message's text from a message_created
// task payload (empty for other events), for the message_content condition.
func messageContent(task arcontracts.EvaluateTask) string {
	if task.Event != "message.created" || len(task.Data) == 0 {
		return ""
	}
	var p struct {
		Text string `json:"text"`
	}
	_ = json.Unmarshal(task.Data, &p)
	return p.Text
}

// hydrate loads the conversation and its contact into an EvalContext. Conditions
// resolve against the live conversation — for message_created too — except the
// message_content condition, which uses the triggering message's text.
func (e *Evaluator) hydrate(ctx context.Context, conversationID, msgContent string) (entity.EvalContext, error) {
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
		MessageContent:  msgContent,
	}
	if conv.ContactID != "" {
		if contact, cerr := e.contacts.FindByID(ctx, conv.ContactID); cerr == nil && contact != nil {
			ec.ContactPhone = contact.Phone
		}
	}
	return ec, nil
}

func (e *Evaluator) log(ctx context.Context, task arcontracts.EvaluateTask, ruleID string, event entity.RuleEvent, actionType string, status entity.EvalStatus, errSummary string) {
	_ = e.logs.Create(ctx, &entity.RuleEvaluationLog{
		ID:             shared.NewID(),
		TenantID:       task.TenantID,
		RuleID:         ruleID,
		Event:          event,
		ConversationID: task.ConversationID,
		ActionType:     actionType,
		Status:         status,
		ErrorSummary:   errSummary,
		CreatedAt:      e.clock.Now(),
	})
}

func dedupKey(tenantID, ruleID, eventID string) string {
	return "rule:fired:" + tenantID + ":" + ruleID + ":" + eventID
}

func summarize(err error) string {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}
