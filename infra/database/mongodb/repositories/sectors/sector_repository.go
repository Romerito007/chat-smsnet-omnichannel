// Package sectors is the Mongo implementation of the sector repository. Every
// operation is scoped by the tenant taken from the context.
package sectors

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sectors/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// Repository implements repository.SectorRepository over MongoDB.
type Repository struct {
	coll *mongo.Collection
}

// New builds the repository.
func New(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("sectors")}
}

func (r *Repository) Create(ctx context.Context, s *entity.Sector) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toModel(s))
	return mongodb.MapError(err)
}

func (r *Repository) Update(ctx context.Context, s *entity.Sector) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": s.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":           s.Name,
			"description":    s.Description,
			"enabled":        s.Enabled,
			"business_hours": s.BusinessHours,
			"updated_at":     s.UpdatedAt,
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

func (r *Repository) FindByID(ctx context.Context, id string) (*entity.Sector, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Sector
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *Repository) List(ctx context.Context, page shared.PageRequest) ([]*entity.Sector, error) {
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
	var out []*entity.Sector
	for c.Next(ctx) {
		var m models.Sector
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toModel(s *entity.Sector) models.Sector {
	m := models.Sector{
		Name:          s.Name,
		Description:   s.Description,
		Enabled:       s.Enabled,
		BusinessHours: s.BusinessHours,
	}
	m.ID = s.ID
	m.TenantID = s.TenantID
	m.CreatedAt = s.CreatedAt
	m.UpdatedAt = s.UpdatedAt
	return m
}

func toEntity(m *models.Sector) *entity.Sector {
	return &entity.Sector{
		ID:            m.ID,
		TenantID:      m.TenantID,
		Name:          m.Name,
		Description:   m.Description,
		Enabled:       m.Enabled,
		BusinessHours: m.BusinessHours,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

var _ repository.SectorRepository = (*Repository)(nil)
