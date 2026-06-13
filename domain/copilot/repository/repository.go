// Package repository declares the copilot persistence contracts.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ConfigRepository persists the per-tenant copilot config (one per tenant).
type ConfigRepository interface {
	Create(ctx context.Context, c *entity.AIConfig) error
	Update(ctx context.Context, c *entity.AIConfig) error
	// FindByTenant returns the tenant's config or a not_found error.
	FindByTenant(ctx context.Context) (*entity.AIConfig, error)
}

// LogRepository persists AI usage logs (summaries only, never raw prompts).
type LogRepository interface {
	Create(ctx context.Context, l *entity.AILog) error
	ListByConversation(ctx context.Context, conversationID string, page shared.PageRequest) ([]*entity.AILog, error)
}

// AssistantRepository persists copilot assistants (many per tenant).
type AssistantRepository interface {
	Create(ctx context.Context, a *entity.Assistant) error
	Update(ctx context.Context, a *entity.Assistant) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.Assistant, error)
	List(ctx context.Context) ([]*entity.Assistant, error)
	// FindByChannelType returns an enabled assistant serving the channel type, or a
	// not_found error.
	FindByChannelType(ctx context.Context, channelType string) (*entity.Assistant, error)
	// CountByISPProfile counts assistants referencing the given ISP profile id
	// (used to block deleting a profile in use).
	CountByISPProfile(ctx context.Context, ispProfileID string) (int, error)
}
