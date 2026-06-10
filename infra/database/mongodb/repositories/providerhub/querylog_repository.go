package providerhub

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// QueryLogRepository implements repository.QueryLogRepository.
type QueryLogRepository struct {
	coll *mongo.Collection
}

// NewQueryLogRepository builds the repository.
func NewQueryLogRepository(db *mongo.Database) *QueryLogRepository {
	return &QueryLogRepository{coll: db.Collection("provider_query_logs")}
}

func (r *QueryLogRepository) Create(ctx context.Context, l *entity.ProviderQueryLog) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, models.ProviderQueryLog{
		ID:             l.ID,
		TenantID:       l.TenantID,
		UserID:         l.UserID,
		ContactID:      l.ContactID,
		ConversationID: l.ConversationID,
		QueryType:      string(l.QueryType),
		Status:         string(l.Status),
		LatencyMs:      l.LatencyMs,
		ErrorSummary:   l.ErrorSummary,
		CreatedAt:      l.CreatedAt,
	})
	return mongodb.MapError(err)
}

var _ repository.QueryLogRepository = (*QueryLogRepository)(nil)
