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
	trigger := triggerMessageFrom(task)
	ruleEvents := applicableRuleEvents(task, trigger)
	if len(ruleEvents) == 0 {
		return nil // not an event rules react to
	}
	ctx = shared.WithTenant(ctx, task.TenantID)

	// A rule belongs to exactly one event, so the per-event lists are disjoint; the
	// union is the candidate set (interactive_reply_received rides message.created).
	var rules []*entity.AutomationRule
	for _, re := range ruleEvents {
		rs, err := e.rules.ListEnabledByEvent(ctx, re)
		if err != nil {
			return err
		}
		rules = append(rules, rs...)
	}
	if len(rules) == 0 {
		return nil // no rules for this event → nothing to do (cheap)
	}

	ec, err := e.hydrate(ctx, task.ConversationID, trigger)
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
	live, err := e.hydrate(ctx, task.ConversationID, trigger)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return nil
		}
		return err
	}

	for _, rule := range matched {
		if !rule.Matches(live) {
			e.log(ctx, task, rule.ID, rule.Event, "", entity.EvalSkippedStale, "")
			continue
		}
		// Dedup CLAIMS the (rule, event_id) firing BEFORE running actions, so an
		// Asynq retry of the same task finds the key claimed and skips — never
		// re-running a side-effectful action (e.g. send_message twice).
		if e.dedup != nil {
			allowed, derr := e.dedup.Allow(ctx, dedupKey(task.TenantID, rule.ID, task.EventID))
			if derr == nil && !allowed {
				e.log(ctx, task, rule.ID, rule.Event, "", entity.EvalSkippedDedup, "")
				continue
			}
		}
		e.fire(ctx, task, rule)
	}
	return nil
}

// fire runs a matching rule's actions in declared order, best-effort: a failing
// action does not abort the rest. Each action's outcome is logged on its own row.
// The wire event is the rule's own event (a rule belongs to exactly one).
func (e *Evaluator) fire(ctx context.Context, task arcontracts.EvaluateTask, rule *entity.AutomationRule) {
	ac := ActionContext{
		TenantID:       task.TenantID,
		ConversationID: task.ConversationID,
		RuleID:         rule.ID,
		EventWire:      string(rule.Event),
		Data:           task.Data,
		Depth:          task.Depth,
	}
	for _, a := range rule.Actions {
		status, summary := classify(e.actions.Run(ctx, a, ac))
		e.log(ctx, task, rule.ID, rule.Event, string(a.Type), status, summary)
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

// triggerMessage is the per-message data a message.created event carries for
// condition matching: the text (message_content), the message_type, and the chosen
// interactive_reply id. All empty for non-message events.
type triggerMessage struct {
	Text               string
	MessageType        string
	InteractiveReplyID string
}

// triggerMessageFrom extracts the triggering message's fields from a message.created
// task payload (empty for other events).
func triggerMessageFrom(task arcontracts.EvaluateTask) triggerMessage {
	if task.Event != "message.created" || len(task.Data) == 0 {
		return triggerMessage{}
	}
	var p struct {
		Text             string `json:"text"`
		MessageType      string `json:"message_type"`
		InteractiveReply *struct {
			ID string `json:"id"`
		} `json:"interactive_reply"`
	}
	_ = json.Unmarshal(task.Data, &p)
	tm := triggerMessage{Text: p.Text, MessageType: p.MessageType}
	if p.InteractiveReply != nil {
		tm.InteractiveReplyID = p.InteractiveReply.ID
	}
	return tm
}

// applicableRuleEvents maps a task to the rule events it can fire. message.created
// always maps to message_created and, when the message is an interactive reply,
// ALSO to interactive_reply_received (a rule belongs to one event, so the union is
// non-overlapping). Other internal events map 1:1.
func applicableRuleEvents(task arcontracts.EvaluateTask, tm triggerMessage) []entity.RuleEvent {
	base, ok := entity.RuleEventForInternal(task.Event)
	if !ok {
		return nil
	}
	events := []entity.RuleEvent{base}
	if base == entity.EventMessageCreated && tm.MessageType == "interactive_reply" {
		events = append(events, entity.EventInteractiveReplyReceived)
	}
	return events
}

// hydrate loads the conversation and its contact into an EvalContext. Conditions
// resolve against the live conversation — for message events too — except the
// message_content and interactive_reply_id conditions, which use the triggering
// message's fields.
func (e *Evaluator) hydrate(ctx context.Context, conversationID string, tm triggerMessage) (entity.EvalContext, error) {
	conv, err := e.conversations.FindByID(ctx, conversationID)
	if err != nil {
		return entity.EvalContext{}, err
	}
	ec := entity.EvalContext{
		Status:             string(conv.Status),
		Channel:            conv.Channel,
		AssignedAgentID:    conv.AssignedTo,
		SectorID:           conv.SectorID,
		QueueID:            conv.QueueID,
		Priority:           string(conv.Priority),
		Tags:               conv.Tags,
		MessageContent:     tm.Text,
		InteractiveReplyID: tm.InteractiveReplyID,
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
