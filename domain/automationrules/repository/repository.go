// Package repository declares the automation-rules persistence contracts.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// RuleRepository persists automation rules (many per tenant).
type RuleRepository interface {
	Create(ctx context.Context, r *entity.AutomationRule) error
	Update(ctx context.Context, r *entity.AutomationRule) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.AutomationRule, error)
	List(ctx context.Context) ([]*entity.AutomationRule, error)
	// ListEnabledByEvent returns the tenant's enabled rules for an event (used by
	// the async evaluator).
	ListEnabledByEvent(ctx context.Context, event entity.RuleEvent) ([]*entity.AutomationRule, error)
	// FindOneByWebhook returns a rule referencing the given webhook id (any), used
	// to block deleting a webhook in use. nil rule + nil error when none.
	FindOneByWebhook(ctx context.Context, webhookID string) (*entity.AutomationRule, error)
}

// LogRepository persists the minimal rule-evaluation log (no event payload).
type LogRepository interface {
	Create(ctx context.Context, l *entity.RuleEvaluationLog) error
	// ListByRule returns a rule's evaluation logs (keyset pagination).
	ListByRule(ctx context.Context, ruleID string, page shared.PageRequest) ([]*entity.RuleEvaluationLog, error)
}
