package automationrules

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// LogRepository implements repository.LogRepository over the rule_evaluation_logs
// collection. It stores no event payload.
type LogRepository struct {
	coll *mongo.Collection
}

// NewLogRepository builds the repository.
func NewLogRepository(db *mongo.Database) *LogRepository {
	return &LogRepository{coll: db.Collection("rule_evaluation_logs")}
}

func (r *LogRepository) Create(ctx context.Context, l *entity.RuleEvaluationLog) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, models.RuleEvaluationLog{
		ID:             l.ID,
		TenantID:       l.TenantID,
		RuleID:         l.RuleID,
		Event:          string(l.Event),
		ConversationID: l.ConversationID,
		ActionType:     l.ActionType,
		Status:         string(l.Status),
		ErrorSummary:   l.ErrorSummary,
		CreatedAt:      l.CreatedAt,
	})
	return mongodb.MapError(err)
}

func (r *LogRepository) ListByRule(ctx context.Context, ruleID string, page shared.PageRequest) ([]*entity.RuleEvaluationLog, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	filter := mongodb.ApplyKeyset(bson.M{"tenant_id": tenantID, "rule_id": ruleID}, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.RuleEvaluationLog
	for c.Next(ctx) {
		var m models.RuleEvaluationLog
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, &entity.RuleEvaluationLog{
			ID:             m.ID,
			TenantID:       m.TenantID,
			RuleID:         m.RuleID,
			Event:          entity.RuleEvent(m.Event),
			ConversationID: m.ConversationID,
			ActionType:     m.ActionType,
			Status:         entity.EvalStatus(m.Status),
			ErrorSummary:   m.ErrorSummary,
			CreatedAt:      m.CreatedAt,
		})
	}
	return out, mongodb.MapError(c.Err())
}

var _ repository.LogRepository = (*LogRepository)(nil)
