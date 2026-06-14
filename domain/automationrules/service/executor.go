package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ConversationMessenger injects automation-authored messages, reusing the normal
// send pipeline (persistMessage → message_created → webhooks). Implemented by the
// conversations Service. The message is authored as SenderType=automation with
// SenderID=ruleID, so it shows as "System Automation" and carries origin=automation.
type ConversationMessenger interface {
	SendAutomationMessage(ctx context.Context, conversationID, ruleID, text string) error
}

// ActionContext is everything an action needs to run.
type ActionContext struct {
	TenantID       string
	ConversationID string
	RuleID         string
	EventWire      string          // rule wire event name (send_webhook delivery event)
	Data           json.RawMessage // original event payload (send_webhook data)
}

// ActionRunner executes one rule action against the system.
type ActionRunner interface {
	Run(ctx context.Context, action entity.Action, ac ActionContext) error
}

// Executor runs rule actions. Every action runs under an OriginAutomation context
// so any lifecycle event it produces is suppressed by the evaluator (anti-loop).
// Step 1 supports send_webhook and send_message; the rest of the catalog is added
// in step 2.
type Executor struct {
	webhooks  WebhookEmitter
	messenger ConversationMessenger
}

// NewExecutor builds the executor.
func NewExecutor(webhooks WebhookEmitter, messenger ConversationMessenger) *Executor {
	return &Executor{webhooks: webhooks, messenger: messenger}
}

// Run dispatches one action by type. The context is tagged OriginAutomation so the
// events this action emits never re-trigger automation rules.
func (x *Executor) Run(ctx context.Context, action entity.Action, ac ActionContext) error {
	ctx = shared.WithRuleOrigin(ctx, shared.OriginAutomation)
	switch action.Type {
	case entity.ActionSendWebhook:
		return x.webhooks.EmitTo(ctx, ac.TenantID, action.Param("webhook_id"), ac.EventWire, ac.Data)
	case entity.ActionSendMessage:
		if x.messenger == nil {
			return fmt.Errorf("send_message: messenger not configured")
		}
		return x.messenger.SendAutomationMessage(ctx, ac.ConversationID, ac.RuleID, action.Param("text"))
	default:
		return fmt.Errorf("unsupported action type %q", action.Type)
	}
}

var _ ActionRunner = (*Executor)(nil)
