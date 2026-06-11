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

// CannedResponseRepository implements repository.CannedResponseRepository.
type CannedResponseRepository struct {
	coll *mongo.Collection
}

// NewCannedResponseRepository builds the repository.
func NewCannedResponseRepository(db *mongo.Database) *CannedResponseRepository {
	return &CannedResponseRepository{coll: db.Collection("canned_responses")}
}

func (r *CannedResponseRepository) Create(ctx context.Context, c *entity.CannedResponse) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toCannedModel(c))
	return mongodb.MapError(err)
}

func (r *CannedResponseRepository) Update(ctx context.Context, c *entity.CannedResponse) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": c.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"sector_ids": c.SectorIDs,
			"shortcut":   c.Shortcut,
			"title":      c.Title,
			"body":       c.Body,
			"enabled":    c.Enabled,
			"updated_at": c.UpdatedAt,
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

func (r *CannedResponseRepository) Delete(ctx context.Context, id string) error {
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

func (r *CannedResponseRepository) FindByID(ctx context.Context, id string) (*entity.CannedResponse, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.CannedResponse
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toCannedEntity(&m), nil
}

func (r *CannedResponseRepository) FindByShortcut(ctx context.Context, shortcut string) (*entity.CannedResponse, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.CannedResponse
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "shortcut": shortcut}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toCannedEntity(&m), nil
}

func (r *CannedResponseRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.CannedResponse, error) {
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
	var out []*entity.CannedResponse
	for c.Next(ctx) {
		var m models.CannedResponse
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toCannedEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toCannedModel(c *entity.CannedResponse) models.CannedResponse {
	m := models.CannedResponse{
		SectorIDs: c.SectorIDs,
		Shortcut:  c.Shortcut,
		Title:     c.Title,
		Body:      c.Body,
		Enabled:   c.Enabled,
	}
	m.ID = c.ID
	m.TenantID = c.TenantID
	m.CreatedAt = c.CreatedAt
	m.UpdatedAt = c.UpdatedAt
	return m
}

func toCannedEntity(m *models.CannedResponse) *entity.CannedResponse {
	return &entity.CannedResponse{
		ID:        m.ID,
		TenantID:  m.TenantID,
		SectorIDs: m.SectorIDs,
		Shortcut:  m.Shortcut,
		Title:     m.Title,
		Body:      m.Body,
		Enabled:   m.Enabled,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

var _ repository.CannedResponseRepository = (*CannedResponseRepository)(nil)
