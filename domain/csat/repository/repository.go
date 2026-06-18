// Package repository declares the CSAT persistence contracts.
package repository

import (
	"context"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/csat/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// SurveyRepository persists CSAT surveys (tenant-scoped from context).
type SurveyRepository interface {
	Create(ctx context.Context, s *entity.CSATSurvey) error
	Update(ctx context.Context, s *entity.CSATSurvey) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*entity.CSATSurvey, error)
	// FindByIDs returns the tenant's surveys for the given ids (batch lookup for
	// name enrichment); missing ids are simply absent.
	FindByIDs(ctx context.Context, ids []string) ([]*entity.CSATSurvey, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.CSATSurvey, error)
	// ListEnabled returns every enabled survey for the tenant (for matching on
	// close).
	ListEnabled(ctx context.Context) ([]*entity.CSATSurvey, error)
}

// ResponseRepository persists CSAT responses.
type ResponseRepository interface {
	Create(ctx context.Context, r *entity.CSATResponse) error
	Update(ctx context.Context, r *entity.CSATResponse) error
	FindByID(ctx context.Context, id string) (*entity.CSATResponse, error)
	// FindByToken looks up a response by its public token, WITHOUT a tenant scope
	// (the public answer endpoint has no tenant context; the record carries it).
	FindByToken(ctx context.Context, token string) (*entity.CSATResponse, error)
	// FindByConversation returns the response for a conversation, used to avoid
	// re-sending to the same conversation.
	FindByConversation(ctx context.Context, conversationID string) (*entity.CSATResponse, error)
	List(ctx context.Context, page shared.PageRequest) ([]*entity.CSATResponse, error)
}
