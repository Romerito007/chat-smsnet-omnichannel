// Package dealtasks is the Mongo implementation of the deal-task repository. Every
// operation is scoped by the tenant from the context.
package dealtasks

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// Repository implements repository.TaskRepository over MongoDB.
type Repository struct {
	coll *mongo.Collection
}

// New builds the repository.
func New(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("deal_tasks")}
}

func (r *Repository) Create(ctx context.Context, t *entity.DealTask) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toModel(t))
	return mongodb.MapError(err)
}

func (r *Repository) Update(ctx context.Context, t *entity.DealTask) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": t.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"title":        t.Title,
			"description":  t.Description,
			"due_date":     t.DueDate,
			"assigned_to":  t.AssignedTo,
			"status":       string(t.Status),
			"completed_at": t.CompletedAt,
			"updated_at":   t.UpdatedAt,
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

func (r *Repository) FindByID(ctx context.Context, id string) (*entity.DealTask, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.DealTask
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *Repository) ListByDeal(ctx context.Context, dealID string, page shared.PageRequest) ([]*entity.DealTask, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	return r.find(ctx, bson.M{"tenant_id": tenantID, "deal_id": dealID}, page)
}

func (r *Repository) List(ctx context.Context, f contracts.ListFilter, page shared.PageRequest) ([]*entity.DealTask, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	base := bson.M{"tenant_id": tenantID}
	if f.AssignedTo != "" {
		base["assigned_to"] = f.AssignedTo
	}
	if f.Status != "" {
		base["status"] = f.Status
	}
	if f.DueBefore != nil {
		base["due_date"] = bson.M{"$lte": *f.DueBefore}
	}
	return r.find(ctx, base, page)
}

func (r *Repository) find(ctx context.Context, base bson.M, page shared.PageRequest) ([]*entity.DealTask, error) {
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	filter := mongodb.ApplyKeyset(base, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.DealTask
	for c.Next(ctx) {
		var m models.DealTask
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toModel(t *entity.DealTask) models.DealTask {
	return models.DealTask{
		ID: t.ID, TenantID: t.TenantID, DealID: t.DealID, Title: t.Title,
		Description: t.Description, DueDate: t.DueDate, AssignedTo: t.AssignedTo,
		Status: string(t.Status), CompletedAt: t.CompletedAt, CreatedBy: t.CreatedBy,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func toEntity(m *models.DealTask) *entity.DealTask {
	return &entity.DealTask{
		ID: m.ID, TenantID: m.TenantID, DealID: m.DealID, Title: m.Title,
		Description: m.Description, DueDate: m.DueDate, AssignedTo: m.AssignedTo,
		Status: entity.Status(m.Status), CompletedAt: m.CompletedAt, CreatedBy: m.CreatedBy,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

var _ repository.TaskRepository = (*Repository)(nil)
