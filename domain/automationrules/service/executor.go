package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ErrBudgetExceeded is returned by a message/attachment action when the
// per-conversation automation-message fuse is tripped. The evaluator logs it as
// skipped_budget (it is a safety breaker, never a hard error).
var ErrBudgetExceeded = errors.New("automation message budget exceeded")

// ConversationOps is the set of conversation mutations the action catalog needs.
// Implemented by the conversations Service (automation-facing, no agent-visibility
// check). A missing referenced entity returns a not_found error → skipped_missing_ref.
type ConversationOps interface {
	SendAutomationMessage(ctx context.Context, conversationID, ruleID, text string) error
	SendAutomationAttachment(ctx context.Context, conversationID, ruleID, attachmentID string) error
	SendAutomationInteractive(ctx context.Context, conversationID, ruleID, interactiveJSON string) error
	AutomationAssignAgent(ctx context.Context, conversationID, agentID string) error
	AutomationAssignTeam(ctx context.Context, conversationID, sectorID string) error
	AutomationRemoveAgent(ctx context.Context, conversationID string) error
	AutomationRemoveTeam(ctx context.Context, conversationID string) error
	AutomationAddTag(ctx context.Context, conversationID, tagID string) error
	AutomationRemoveTag(ctx context.Context, conversationID, tagID string) error
	AutomationChangePriority(ctx context.Context, conversationID, priority string) error
	AutomationResolve(ctx context.Context, conversationID string) error
	AutomationOpen(ctx context.Context, conversationID string) error
	AutomationMarkPending(ctx context.Context, conversationID string) error
}

// DealOps is the deal mutation the action catalog needs. Implemented by the deals
// service (automation-facing). Moving is idempotent and only touches existing deals
// linked to the conversation — none is created.
type DealOps interface {
	AutomationMoveDealStage(ctx context.Context, conversationID, pipelineID, stageID string) error
}

// BudgetLimiter is the LAYER-2 safety fuse: it caps automation message/attachment
// actions per conversation per window (not flow control — only a breaker for a
// buggy integrator that echoes forever). Fail-open on backend errors.
type BudgetLimiter interface {
	AllowAutomationMessage(ctx context.Context, conversationID string) (bool, error)
}

// ActionContext is everything an action needs to run.
type ActionContext struct {
	TenantID       string
	ConversationID string
	RuleID         string
	EventWire      string          // rule wire event name (send_webhook delivery event)
	Data           json.RawMessage // original event payload (send_webhook data)
	Depth          int             // causal depth of the triggering event
}

// ActionRunner executes one rule action against the system.
type ActionRunner interface {
	Run(ctx context.Context, action entity.Action, ac ActionContext) error
}

// Executor runs rule actions. Every action runs under an OriginAutomation context
// (anti-loop layer 1) with the causal depth incremented (layer 3), so any
// lifecycle event it produces is suppressed/bounded by the evaluator.
type Executor struct {
	webhooks WebhookEmitter
	conv     ConversationOps
	deals    DealOps
	budget   BudgetLimiter
}

// NewExecutor builds the executor. The deal ops are optional (a nil one makes
// move_deal_stage a no-op).
func NewExecutor(webhooks WebhookEmitter, conv ConversationOps, deals DealOps, budget BudgetLimiter) *Executor {
	return &Executor{webhooks: webhooks, conv: conv, deals: deals, budget: budget}
}

// Run dispatches one action by type. The context is tagged origin=automation and
// its causal depth is incremented, so events this action emits never re-trigger
// rules (origin) and a runaway in-process chain is bounded (depth).
func (x *Executor) Run(ctx context.Context, action entity.Action, ac ActionContext) error {
	ctx = shared.WithRuleOrigin(ctx, shared.OriginAutomation)
	ctx = shared.WithRuleDepth(ctx, ac.Depth+1)

	switch action.Type {
	case entity.ActionSendWebhook:
		return x.webhooks.EmitTo(ctx, ac.TenantID, action.Param("webhook_id"), ac.EventWire, ac.Data)

	case entity.ActionSendMessage:
		if err := x.checkBudget(ctx, ac.ConversationID); err != nil {
			return err
		}
		return x.requireConv().SendAutomationMessage(ctx, ac.ConversationID, ac.RuleID, action.Param("text"))
	case entity.ActionSendAttachment:
		if err := x.checkBudget(ctx, ac.ConversationID); err != nil {
			return err
		}
		return x.requireConv().SendAutomationAttachment(ctx, ac.ConversationID, ac.RuleID, action.Param("attachment_id"))
	case entity.ActionSendInteractive:
		if err := x.checkBudget(ctx, ac.ConversationID); err != nil {
			return err
		}
		return x.requireConv().SendAutomationInteractive(ctx, ac.ConversationID, ac.RuleID, action.Param("interactive"))

	case entity.ActionAssignAgent:
		return x.requireConv().AutomationAssignAgent(ctx, ac.ConversationID, action.Param("agent_id"))
	case entity.ActionAssignTeam:
		return x.requireConv().AutomationAssignTeam(ctx, ac.ConversationID, action.Param("sector_id"))
	case entity.ActionRemoveAssignedAgent:
		return x.requireConv().AutomationRemoveAgent(ctx, ac.ConversationID)
	case entity.ActionRemoveAssignedTeam:
		return x.requireConv().AutomationRemoveTeam(ctx, ac.ConversationID)
	case entity.ActionAddTag:
		return x.requireConv().AutomationAddTag(ctx, ac.ConversationID, action.Param("tag_id"))
	case entity.ActionRemoveTag:
		return x.requireConv().AutomationRemoveTag(ctx, ac.ConversationID, action.Param("tag_id"))
	case entity.ActionChangePriority:
		return x.requireConv().AutomationChangePriority(ctx, ac.ConversationID, action.Param("priority"))
	case entity.ActionResolveConversation:
		return x.requireConv().AutomationResolve(ctx, ac.ConversationID)
	case entity.ActionOpenConversation:
		return x.requireConv().AutomationOpen(ctx, ac.ConversationID)
	case entity.ActionMarkPending:
		return x.requireConv().AutomationMarkPending(ctx, ac.ConversationID)

	case entity.ActionMoveDealStage:
		return x.requireDeals().AutomationMoveDealStage(ctx, ac.ConversationID, action.Param("pipeline_id"), action.Param("stage_id"))

	default:
		return fmt.Errorf("unsupported action type %q", action.Type)
	}
}

// checkBudget trips the layer-2 fuse for message-creating actions. Fail-open.
func (x *Executor) checkBudget(ctx context.Context, conversationID string) error {
	if x.budget == nil {
		return nil
	}
	allowed, err := x.budget.AllowAutomationMessage(ctx, conversationID)
	if err != nil {
		return nil // fail-open: a limiter outage never blocks a legitimate send
	}
	if !allowed {
		return ErrBudgetExceeded
	}
	return nil
}

func (x *Executor) requireConv() ConversationOps {
	if x.conv == nil {
		return noopConvOps{}
	}
	return x.conv
}

func (x *Executor) requireDeals() DealOps {
	if x.deals == nil {
		return noopDealOps{}
	}
	return x.deals
}

var _ ActionRunner = (*Executor)(nil)

// noopConvOps is a defensive fallback when no conversation ops are wired (tests).
type noopConvOps struct{}

func (noopConvOps) SendAutomationMessage(context.Context, string, string, string) error { return nil }
func (noopConvOps) SendAutomationInteractive(context.Context, string, string, string) error {
	return nil
}
func (noopConvOps) SendAutomationAttachment(context.Context, string, string, string) error {
	return nil
}
func (noopConvOps) AutomationAssignAgent(context.Context, string, string) error    { return nil }
func (noopConvOps) AutomationAssignTeam(context.Context, string, string) error     { return nil }
func (noopConvOps) AutomationRemoveAgent(context.Context, string) error            { return nil }
func (noopConvOps) AutomationRemoveTeam(context.Context, string) error             { return nil }
func (noopConvOps) AutomationAddTag(context.Context, string, string) error         { return nil }
func (noopConvOps) AutomationRemoveTag(context.Context, string, string) error      { return nil }
func (noopConvOps) AutomationChangePriority(context.Context, string, string) error { return nil }
func (noopConvOps) AutomationResolve(context.Context, string) error                { return nil }
func (noopConvOps) AutomationOpen(context.Context, string) error                   { return nil }
func (noopConvOps) AutomationMarkPending(context.Context, string) error            { return nil }

// noopDealOps is the fallback when no deal ops are wired (tests / deals disabled).
type noopDealOps struct{}

func (noopDealOps) AutomationMoveDealStage(context.Context, string, string, string) error {
	return nil
}
