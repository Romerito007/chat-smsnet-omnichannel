package automation

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// RunRepository implements repository.RunRepository.
type RunRepository struct {
	coll *mongo.Collection
}

// NewRunRepository builds the repository.
func NewRunRepository(db *mongo.Database) *RunRepository {
	return &RunRepository{coll: db.Collection("automation_runs")}
}

func (r *RunRepository) Create(ctx context.Context, run *entity.AutomationRun) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, runToModel(run))
	return mongodb.MapError(err)
}

func (r *RunRepository) Update(ctx context.Context, run *entity.AutomationRun) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": run.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"external_run_id": run.ExternalRunID,
			"status":          string(run.Status),
			"output":          run.Output,
			"error":           run.Error,
			"updated_at":      run.UpdatedAt,
		}},
	)
	if err != nil {
		return mongodb.MapError(err)
	}
	if res.MatchedCount == 0 {
		return mongodb.MapError(mongo.ErrNoDocuments)
	}
	return nil
}

func (r *RunRepository) FindByID(ctx context.Context, id string) (*entity.AutomationRun, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.AutomationRun
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return runToEntity(&m), nil
}

func (r *RunRepository) FindByExternalRunID(ctx context.Context, externalRunID string) (*entity.AutomationRun, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.AutomationRun
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "external_run_id": externalRunID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return runToEntity(&m), nil
}

func (r *RunRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.AutomationRun, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	filter := mongodb.ApplyKeyset(bson.M{"tenant_id": tenantID}, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.AutomationRun
	for c.Next(ctx) {
		var m models.AutomationRun
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, runToEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func runToModel(r *entity.AutomationRun) models.AutomationRun {
	return models.AutomationRun{
		ID:             r.ID,
		TenantID:       r.TenantID,
		ConversationID: r.ConversationID,
		MessageID:      r.MessageID,
		ExternalRunID:  r.ExternalRunID,
		Status:         string(r.Status),
		Input:          r.Input,
		Output:         r.Output,
		Error:          r.Error,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

func runToEntity(m *models.AutomationRun) *entity.AutomationRun {
	return &entity.AutomationRun{
		ID:             m.ID,
		TenantID:       m.TenantID,
		ConversationID: m.ConversationID,
		MessageID:      m.MessageID,
		ExternalRunID:  m.ExternalRunID,
		Status:         entity.RunStatus(m.Status),
		Input:          m.Input,
		Output:         m.Output,
		Error:          m.Error,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

var _ repository.RunRepository = (*RunRepository)(nil)
