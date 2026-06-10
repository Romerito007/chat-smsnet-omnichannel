package copilot

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// LogRepository implements repository.LogRepository.
type LogRepository struct {
	coll *mongo.Collection
}

// NewLogRepository builds the repository.
func NewLogRepository(db *mongo.Database) *LogRepository {
	return &LogRepository{coll: db.Collection("copilot_logs")}
}

func (r *LogRepository) Create(ctx context.Context, l *entity.AILog) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toLogModel(l))
	return mongodb.MapError(err)
}

func (r *LogRepository) ListByConversation(ctx context.Context, conversationID string, page shared.PageRequest) ([]*entity.AILog, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	filter := mongodb.ApplyKeyset(bson.M{"tenant_id": tenantID, "conversation_id": conversationID}, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer c.Close(ctx)
	var out []*entity.AILog
	for c.Next(ctx) {
		var m models.AILog
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toLogEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toLogModel(l *entity.AILog) models.AILog {
	return models.AILog{
		ID:             l.ID,
		TenantID:       l.TenantID,
		UserID:         l.UserID,
		ConversationID: l.ConversationID,
		Provider:       l.Provider,
		Model:          l.Model,
		Action:         string(l.Action),
		InputSummary:   l.InputSummary,
		OutputSummary:  l.OutputSummary,
		TokensInput:    l.TokensInput,
		TokensOutput:   l.TokensOutput,
		EstimatedCost:  l.EstimatedCost,
		Status:         string(l.Status),
		Error:          l.Error,
		CreatedAt:      l.CreatedAt,
	}
}

func toLogEntity(m *models.AILog) *entity.AILog {
	return &entity.AILog{
		ID:             m.ID,
		TenantID:       m.TenantID,
		UserID:         m.UserID,
		ConversationID: m.ConversationID,
		Provider:       m.Provider,
		Model:          m.Model,
		Action:         entity.Action(m.Action),
		InputSummary:   m.InputSummary,
		OutputSummary:  m.OutputSummary,
		TokensInput:    m.TokensInput,
		TokensOutput:   m.TokensOutput,
		EstimatedCost:  m.EstimatedCost,
		Status:         entity.LogStatus(m.Status),
		Error:          m.Error,
		CreatedAt:      m.CreatedAt,
	}
}

var _ repository.LogRepository = (*LogRepository)(nil)
