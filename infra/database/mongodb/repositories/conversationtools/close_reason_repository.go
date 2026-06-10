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

// CloseReasonRepository implements repository.CloseReasonRepository.
type CloseReasonRepository struct {
	coll *mongo.Collection
}

// NewCloseReasonRepository builds the repository.
func NewCloseReasonRepository(db *mongo.Database) *CloseReasonRepository {
	return &CloseReasonRepository{coll: db.Collection("close_reasons")}
}

func (r *CloseReasonRepository) Create(ctx context.Context, c *entity.CloseReason) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toCloseReasonModel(c))
	return mongodb.MapError(err)
}

func (r *CloseReasonRepository) Update(ctx context.Context, c *entity.CloseReason) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": c.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":          c.Name,
			"requires_note": c.RequiresNote,
			"enabled":       c.Enabled,
			"updated_at":    c.UpdatedAt,
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

func (r *CloseReasonRepository) Delete(ctx context.Context, id string) error {
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

func (r *CloseReasonRepository) FindByID(ctx context.Context, id string) (*entity.CloseReason, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.CloseReason
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toCloseReasonEntity(&m), nil
}

func (r *CloseReasonRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.CloseReason, error) {
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
	var out []*entity.CloseReason
	for c.Next(ctx) {
		var m models.CloseReason
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toCloseReasonEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toCloseReasonModel(c *entity.CloseReason) models.CloseReason {
	m := models.CloseReason{Name: c.Name, RequiresNote: c.RequiresNote, Enabled: c.Enabled}
	m.ID = c.ID
	m.TenantID = c.TenantID
	m.CreatedAt = c.CreatedAt
	m.UpdatedAt = c.UpdatedAt
	return m
}

func toCloseReasonEntity(m *models.CloseReason) *entity.CloseReason {
	return &entity.CloseReason{
		ID:           m.ID,
		TenantID:     m.TenantID,
		Name:         m.Name,
		RequiresNote: m.RequiresNote,
		Enabled:      m.Enabled,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

var _ repository.CloseReasonRepository = (*CloseReasonRepository)(nil)
