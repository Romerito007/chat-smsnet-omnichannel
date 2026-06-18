// Package deals is the Mongo implementation of the deals repository. Every
// operation is scoped by the tenant from the context.
package deals

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/deals/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// Repository implements repository.DealRepository over MongoDB.
type Repository struct {
	coll *mongo.Collection
}

// New builds the repository.
func New(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("deals")}
}

func (r *Repository) Create(ctx context.Context, d *entity.Deal) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toModel(d))
	return mongodb.MapError(err)
}

func (r *Repository) Update(ctx context.Context, d *entity.Deal) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": d.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"pipeline_id":         d.PipelineID,
			"stage_id":            d.StageID,
			"contact_id":          d.ContactID,
			"title":               d.Title,
			"value":               d.Value,
			"currency":            d.Currency,
			"assigned_to":         d.AssignedTo,
			"sector_id":           d.SectorID,
			"conversation_ids":    d.ConversationIDs,
			"source":              d.Source,
			"status":              string(d.Status),
			"lost_reason":         d.LostReason,
			"expected_close_date": d.ExpectedCloseDate,
			"stage_changed_at":    d.StageChangedAt,
			"closed_at":           d.ClosedAt,
			"updated_at":          d.UpdatedAt,
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

func (r *Repository) FindByID(ctx context.Context, id string) (*entity.Deal, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Deal
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *Repository) List(ctx context.Context, f contracts.ListFilter, vis contracts.Visibility, page shared.PageRequest) ([]*entity.Deal, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	base := bson.M{"tenant_id": tenantID}
	if f.PipelineID != "" {
		base["pipeline_id"] = f.PipelineID
	}
	if f.StageID != "" {
		base["stage_id"] = f.StageID
	}
	if f.AssignedTo != "" {
		base["assigned_to"] = f.AssignedTo
	}
	if f.ContactID != "" {
		base["contact_id"] = f.ContactID
	}
	if f.Status != "" {
		base["status"] = f.Status
	}
	if f.Q != "" {
		base["title"] = bson.M{"$regex": f.Q, "$options": "i"}
	}
	// Visibility: when not all-scope, restrict to assigned-to-me OR my sectors.
	if !vis.All {
		base["$or"] = bson.A{
			bson.M{"assigned_to": vis.UserID},
			bson.M{"sector_id": bson.M{"$in": nonEmpty(vis.SectorIDs)}},
		}
	}
	filter := mongodb.ApplyKeyset(base, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.Deal
	for c.Next(ctx) {
		var m models.Deal
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func (r *Repository) CountByStage(ctx context.Context, pipelineID, stageID string) (int, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, err
	}
	n, err := r.coll.CountDocuments(ctx, bson.M{"tenant_id": tenantID, "pipeline_id": pipelineID, "stage_id": stageID})
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	return int(n), nil
}

func toModel(d *entity.Deal) models.Deal {
	m := models.Deal{
		PipelineID: d.PipelineID, StageID: d.StageID, ContactID: d.ContactID, Title: d.Title,
		Value: d.Value, Currency: d.Currency, AssignedTo: d.AssignedTo, SectorID: d.SectorID,
		ConversationIDs: d.ConversationIDs, Source: d.Source, Status: string(d.Status),
		LostReason: d.LostReason, ExpectedCloseDate: d.ExpectedCloseDate,
		StageChangedAt: d.StageChangedAt, ClosedAt: d.ClosedAt,
	}
	m.ID = d.ID
	m.TenantID = d.TenantID
	m.CreatedAt = d.CreatedAt
	m.UpdatedAt = d.UpdatedAt
	return m
}

func toEntity(m *models.Deal) *entity.Deal {
	return &entity.Deal{
		ID: m.ID, TenantID: m.TenantID, PipelineID: m.PipelineID, StageID: m.StageID,
		ContactID: m.ContactID, Title: m.Title, Value: m.Value, Currency: m.Currency,
		AssignedTo: m.AssignedTo, SectorID: m.SectorID, ConversationIDs: m.ConversationIDs,
		Source: m.Source, Status: entity.Status(m.Status), LostReason: m.LostReason,
		ExpectedCloseDate: m.ExpectedCloseDate, StageChangedAt: m.StageChangedAt,
		ClosedAt: m.ClosedAt, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func nonEmpty(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

var _ repository.DealRepository = (*Repository)(nil)
