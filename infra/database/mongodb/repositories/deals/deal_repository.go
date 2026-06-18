// Package deals is the Mongo implementation of the deals repository. Every
// operation is scoped by the tenant from the context.
package deals

import (
	"context"
	"time"

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

// ── sales metrics aggregations ───────────────────────────────────────────────

// salesBase builds the shared match for sales metrics: tenant + optional pipeline +
// visibility ($or assigned-to-me / my sectors when not all-scope).
func salesBase(tenantID string, f contracts.SalesFilter, vis contracts.Visibility) bson.M {
	m := bson.M{"tenant_id": tenantID}
	if f.PipelineID != "" {
		m["pipeline_id"] = f.PipelineID
	}
	if !vis.All {
		m["$or"] = bson.A{
			bson.M{"assigned_to": vis.UserID},
			bson.M{"sector_id": bson.M{"$in": nonEmpty(vis.SectorIDs)}},
		}
	}
	return m
}

// closedPeriod adds a closed_at range to a match (bounds optional).
func closedPeriod(m bson.M, f contracts.SalesFilter) bson.M {
	if f.From.IsZero() && f.To.IsZero() {
		return m
	}
	rng := bson.M{}
	if !f.From.IsZero() {
		rng["$gte"] = f.From
	}
	if !f.To.IsZero() {
		rng["$lte"] = f.To
	}
	m["closed_at"] = rng
	return m
}

// groupCountValue groups by groupBy, summing the count and the value.
func (r *Repository) groupCountValue(ctx context.Context, match bson.M, groupBy string) (map[string]contracts.CountValue, error) {
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{"_id": groupBy, "count": bson.M{"$sum": 1}, "value": bson.M{"$sum": "$value"}}}},
	}
	c, err := r.coll.Aggregate(ctx, pipe)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	out := map[string]contracts.CountValue{}
	for c.Next(ctx) {
		var row struct {
			ID    string  `bson:"_id"`
			Count int     `bson:"count"`
			Value float64 `bson:"value"`
		}
		if err := c.Decode(&row); err != nil {
			return nil, mongodb.MapError(err)
		}
		out[row.ID] = contracts.CountValue{Count: row.Count, Value: row.Value}
	}
	return out, mongodb.MapError(c.Err())
}

func (r *Repository) OpenByStage(ctx context.Context, f contracts.SalesFilter, vis contracts.Visibility) ([]contracts.FunnelStage, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	match := salesBase(tenantID, f, vis)
	match["status"] = string(entity.StatusOpen)
	byStage, err := r.groupCountValue(ctx, match, "$stage_id")
	if err != nil {
		return nil, err
	}
	out := make([]contracts.FunnelStage, 0, len(byStage))
	for id, cv := range byStage {
		out = append(out, contracts.FunnelStage{StageID: id, Count: cv.Count, Value: cv.Value})
	}
	return out, nil
}

func (r *Repository) ClosedTotals(ctx context.Context, status string, f contracts.SalesFilter, vis contracts.Visibility) (contracts.CountValue, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return contracts.CountValue{}, err
	}
	match := salesBase(tenantID, f, vis)
	match["status"] = status
	closedPeriod(match, f)
	byStatus, err := r.groupCountValue(ctx, match, "$status")
	if err != nil {
		return contracts.CountValue{}, err
	}
	return byStatus[status], nil
}

func (r *Repository) OpenByAgent(ctx context.Context, f contracts.SalesFilter, vis contracts.Visibility) (map[string]contracts.CountValue, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	match := salesBase(tenantID, f, vis)
	match["status"] = string(entity.StatusOpen)
	return r.groupCountValue(ctx, match, "$assigned_to")
}

func (r *Repository) ClosedByAgent(ctx context.Context, status string, f contracts.SalesFilter, vis contracts.Visibility) (map[string]contracts.CountValue, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	match := salesBase(tenantID, f, vis)
	match["status"] = status
	closedPeriod(match, f)
	return r.groupCountValue(ctx, match, "$assigned_to")
}

func (r *Repository) AvgCloseSeconds(ctx context.Context, f contracts.SalesFilter, vis contracts.Visibility) (float64, int, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, 0, err
	}
	match := salesBase(tenantID, f, vis)
	match["status"] = string(entity.StatusWon)
	closedPeriod(match, f)
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{
			"_id":   nil,
			"avgMs": bson.M{"$avg": bson.M{"$subtract": bson.A{"$closed_at", "$created_at"}}},
			"count": bson.M{"$sum": 1},
		}}},
	}
	c, err := r.coll.Aggregate(ctx, pipe)
	if err != nil {
		return 0, 0, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	if c.Next(ctx) {
		var row struct {
			AvgMs float64 `bson:"avgMs"`
			Count int     `bson:"count"`
		}
		if err := c.Decode(&row); err != nil {
			return 0, 0, mongodb.MapError(err)
		}
		return row.AvgMs / 1000.0, row.Count, mongodb.MapError(c.Err())
	}
	return 0, 0, mongodb.MapError(c.Err())
}

func (r *Repository) StageDwell(ctx context.Context, now time.Time, f contracts.SalesFilter, vis contracts.Visibility) ([]contracts.StageDwell, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	match := salesBase(tenantID, f, vis)
	match["status"] = string(entity.StatusOpen)
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$stage_id",
			"avgMs": bson.M{"$avg": bson.M{"$subtract": bson.A{now, "$stage_changed_at"}}},
			"count": bson.M{"$sum": 1},
		}}},
	}
	c, err := r.coll.Aggregate(ctx, pipe)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []contracts.StageDwell
	for c.Next(ctx) {
		var row struct {
			ID    string  `bson:"_id"`
			AvgMs float64 `bson:"avgMs"`
			Count int     `bson:"count"`
		}
		if err := c.Decode(&row); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, contracts.StageDwell{StageID: row.ID, OpenCount: row.Count, AvgSeconds: row.AvgMs / 1000.0})
	}
	return out, mongodb.MapError(c.Err())
}

func (r *Repository) StalledOpen(ctx context.Context, before time.Time, limit int, f contracts.SalesFilter, vis contracts.Visibility) ([]*entity.Deal, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	match := salesBase(tenantID, f, vis)
	match["status"] = string(entity.StatusOpen)
	match["stage_changed_at"] = bson.M{"$lt": before}
	opts := options.Find().SetSort(bson.D{{Key: "stage_changed_at", Value: 1}}).SetLimit(int64(limit))
	c, err := r.coll.Find(ctx, match, opts)
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

func nonEmpty(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

var _ repository.DealRepository = (*Repository)(nil)
