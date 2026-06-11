// Package businesshours is the Mongo implementation of the businesshours
// repositories (holidays).
package businesshours

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/businesshours/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// HolidayRepository implements repository.HolidayRepository.
type HolidayRepository struct {
	coll *mongo.Collection
}

// NewHolidayRepository builds the repository.
func NewHolidayRepository(db *mongo.Database) *HolidayRepository {
	return &HolidayRepository{coll: db.Collection("holidays")}
}

func (r *HolidayRepository) Create(ctx context.Context, h *entity.Holiday) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toModel(h))
	return mongodb.MapError(err)
}

func (r *HolidayRepository) Update(ctx context.Context, h *entity.Holiday) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": h.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"date":       h.Date,
			"name":       h.Name,
			"scope":      string(h.Scope),
			"sector_ids": h.SectorIDs,
			"recurring":  h.Recurring,
			"updated_at": h.UpdatedAt,
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

func (r *HolidayRepository) Delete(ctx context.Context, id string) error {
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

func (r *HolidayRepository) FindByID(ctx context.Context, id string) (*entity.Holiday, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Holiday
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *HolidayRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.Holiday, error) {
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

func (r *HolidayRepository) ListAll(ctx context.Context) ([]*entity.Holiday, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	return r.query(ctx, bson.M{"tenant_id": tenantID}, nil)
}

func (r *HolidayRepository) query(ctx context.Context, filter bson.M, opts *options.FindOptions) ([]*entity.Holiday, error) {
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.Holiday
	for c.Next(ctx) {
		var m models.Holiday
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toModel(h *entity.Holiday) models.Holiday {
	m := models.Holiday{Date: h.Date, Name: h.Name, Scope: string(h.Scope), SectorIDs: h.SectorIDs, Recurring: h.Recurring}
	m.ID = h.ID
	m.TenantID = h.TenantID
	m.CreatedAt = h.CreatedAt
	m.UpdatedAt = h.UpdatedAt
	return m
}

func toEntity(m *models.Holiday) *entity.Holiday {
	return &entity.Holiday{
		ID:        m.ID,
		TenantID:  m.TenantID,
		Date:      m.Date,
		Name:      m.Name,
		Scope:     entity.HolidayScope(m.Scope),
		SectorIDs: m.SectorIDs,
		Recurring: m.Recurring,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

var _ repository.HolidayRepository = (*HolidayRepository)(nil)
