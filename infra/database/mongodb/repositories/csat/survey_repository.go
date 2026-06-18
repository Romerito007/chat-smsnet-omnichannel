// Package csat is the Mongo implementation of the CSAT repositories.
package csat

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/csat/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/csat/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// SurveyRepository implements repository.SurveyRepository.
type SurveyRepository struct {
	coll *mongo.Collection
}

// NewSurveyRepository builds the repository.
func NewSurveyRepository(db *mongo.Database) *SurveyRepository {
	return &SurveyRepository{coll: db.Collection("csat_surveys")}
}

func (r *SurveyRepository) Create(ctx context.Context, s *entity.CSATSurvey) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toSurveyModel(s))
	return mongodb.MapError(err)
}

func (r *SurveyRepository) Update(ctx context.Context, s *entity.CSATSurvey) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": s.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":          s.Name,
			"scale":         string(s.Scale),
			"question_text": s.QuestionText,
			"send_on":       string(s.SendOn),
			"sector_ids":    s.SectorIDs,
			"delay_seconds": s.DelaySeconds,
			"enabled":       s.Enabled,
			"updated_at":    s.UpdatedAt,
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

func (r *SurveyRepository) Delete(ctx context.Context, id string) error {
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

func (r *SurveyRepository) FindByID(ctx context.Context, id string) (*entity.CSATSurvey, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.CSATSurvey
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toSurveyEntity(&m), nil
}

func (r *SurveyRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.CSATSurvey, error) {
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
	return r.query(ctx, filter, opts)
}

func (r *SurveyRepository) FindByIDs(ctx context.Context, ids []string) ([]*entity.CSATSurvey, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return r.query(ctx, bson.M{"tenant_id": tenantID, "_id": bson.M{"$in": ids}}, nil)
}

func (r *SurveyRepository) ListEnabled(ctx context.Context) ([]*entity.CSATSurvey, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	return r.query(ctx, bson.M{"tenant_id": tenantID, "enabled": true}, nil)
}

func (r *SurveyRepository) query(ctx context.Context, filter bson.M, opts *options.FindOptions) ([]*entity.CSATSurvey, error) {
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.CSATSurvey
	for c.Next(ctx) {
		var m models.CSATSurvey
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toSurveyEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toSurveyModel(s *entity.CSATSurvey) models.CSATSurvey {
	m := models.CSATSurvey{
		Name:         s.Name,
		Scale:        string(s.Scale),
		QuestionText: s.QuestionText,
		SendOn:       string(s.SendOn),
		SectorIDs:    s.SectorIDs,
		DelaySeconds: s.DelaySeconds,
		Enabled:      s.Enabled,
	}
	m.ID = s.ID
	m.TenantID = s.TenantID
	m.CreatedAt = s.CreatedAt
	m.UpdatedAt = s.UpdatedAt
	return m
}

func toSurveyEntity(m *models.CSATSurvey) *entity.CSATSurvey {
	return &entity.CSATSurvey{
		ID:           m.ID,
		TenantID:     m.TenantID,
		Name:         m.Name,
		Scale:        entity.Scale(m.Scale),
		QuestionText: m.QuestionText,
		SendOn:       entity.SendOn(m.SendOn),
		SectorIDs:    m.SectorIDs,
		DelaySeconds: m.DelaySeconds,
		Enabled:      m.Enabled,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

var _ repository.SurveyRepository = (*SurveyRepository)(nil)
