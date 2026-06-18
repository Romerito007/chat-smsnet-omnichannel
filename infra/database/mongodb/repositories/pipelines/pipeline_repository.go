// Package pipelines is the Mongo implementation of the pipelines repository. Every
// operation is scoped by the tenant from the context.
package pipelines

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/pipelines/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// Repository implements repository.PipelineRepository over MongoDB.
type Repository struct {
	coll *mongo.Collection
}

// New builds the repository.
func New(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("pipelines")}
}

func (r *Repository) Create(ctx context.Context, p *entity.Pipeline) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toModel(p))
	return mongodb.MapError(err)
}

func (r *Repository) Update(ctx context.Context, p *entity.Pipeline) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": p.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":       p.Name,
			"is_default": p.IsDefault,
			"stages":     toStageModels(p.Stages),
			"updated_at": p.UpdatedAt,
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

func (r *Repository) Delete(ctx context.Context, id string) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id, "tenant_id": tenantID})
	if err != nil {
		return mongodb.MapError(err)
	}
	if res.DeletedCount == 0 {
		return mongodb.MapError(mongo.ErrNoDocuments)
	}
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (*entity.Pipeline, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Pipeline
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *Repository) List(ctx context.Context) ([]*entity.Pipeline, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	// Default first, then alphabetical — a stable, friendly order for the selector.
	opts := options.Find().SetSort(bson.D{{Key: "is_default", Value: -1}, {Key: "name", Value: 1}})
	c, err := r.coll.Find(ctx, bson.M{"tenant_id": tenantID}, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.Pipeline
	for c.Next(ctx) {
		var m models.Pipeline
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func (r *Repository) CountByTenant(ctx context.Context) (int, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, err
	}
	n, err := r.coll.CountDocuments(ctx, bson.M{"tenant_id": tenantID})
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	return int(n), nil
}

func (r *Repository) ClearDefault(ctx context.Context, keepID string) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	_, err = r.coll.UpdateMany(ctx,
		bson.M{"tenant_id": tenantID, "_id": bson.M{"$ne": keepID}, "is_default": true},
		bson.M{"$set": bson.M{"is_default": false}},
	)
	return mongodb.MapError(err)
}

func toModel(p *entity.Pipeline) models.Pipeline {
	m := models.Pipeline{Name: p.Name, IsDefault: p.IsDefault, Stages: toStageModels(p.Stages)}
	m.ID = p.ID
	m.TenantID = p.TenantID
	m.CreatedAt = p.CreatedAt
	m.UpdatedAt = p.UpdatedAt
	return m
}

func toStageModels(stages []entity.Stage) []models.PipelineStage {
	out := make([]models.PipelineStage, len(stages))
	for i, st := range stages {
		out[i] = models.PipelineStage{ID: st.ID, Name: st.Name, Order: st.Order, IsWon: st.IsWon, IsLost: st.IsLost, Color: st.Color}
	}
	return out
}

func toEntity(m *models.Pipeline) *entity.Pipeline {
	stages := make([]entity.Stage, len(m.Stages))
	for i, st := range m.Stages {
		stages[i] = entity.Stage{ID: st.ID, Name: st.Name, Order: st.Order, IsWon: st.IsWon, IsLost: st.IsLost, Color: st.Color}
	}
	return &entity.Pipeline{
		ID: m.ID, TenantID: m.TenantID, Name: m.Name, IsDefault: m.IsDefault,
		Stages: stages, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

var _ repository.PipelineRepository = (*Repository)(nil)
