// Package conversationtools is the Mongo implementation of the conversationtools
// repositories (tags, canned responses, close reasons).
package conversationtools

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversationtools/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// TagRepository implements repository.TagRepository.
type TagRepository struct {
	coll *mongo.Collection
}

// NewTagRepository builds the repository.
func NewTagRepository(db *mongo.Database) *TagRepository {
	return &TagRepository{coll: db.Collection("tags")}
}

func (r *TagRepository) Create(ctx context.Context, t *entity.Tag) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toTagModel(t))
	return mongodb.MapError(err)
}

func (r *TagRepository) Update(ctx context.Context, t *entity.Tag) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": t.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":        t.Name,
			"color":       t.Color,
			"description": t.Description,
			"enabled":     t.Enabled,
			"updated_at":  t.UpdatedAt,
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

func (r *TagRepository) Delete(ctx context.Context, id string) error {
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

func (r *TagRepository) FindByID(ctx context.Context, id string) (*entity.Tag, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Tag
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toTagEntity(&m), nil
}

func (r *TagRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.Tag, error) {
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
	defer c.Close(ctx)
	var out []*entity.Tag
	for c.Next(ctx) {
		var m models.Tag
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toTagEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func (r *TagRepository) FindByIDs(ctx context.Context, ids []string) ([]*entity.Tag, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	c, err := r.coll.Find(ctx, bson.M{"tenant_id": tenantID, "_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer c.Close(ctx)
	var out []*entity.Tag
	for c.Next(ctx) {
		var m models.Tag
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toTagEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toTagModel(t *entity.Tag) models.Tag {
	m := models.Tag{Name: t.Name, Color: t.Color, Description: t.Description, Enabled: t.Enabled}
	m.ID = t.ID
	m.TenantID = t.TenantID
	m.CreatedAt = t.CreatedAt
	m.UpdatedAt = t.UpdatedAt
	return m
}

func toTagEntity(m *models.Tag) *entity.Tag {
	return &entity.Tag{
		ID:          m.ID,
		TenantID:    m.TenantID,
		Name:        m.Name,
		Color:       m.Color,
		Description: m.Description,
		Enabled:     m.Enabled,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

var _ repository.TagRepository = (*TagRepository)(nil)
