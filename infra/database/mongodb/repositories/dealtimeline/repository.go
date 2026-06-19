// Package dealtimeline is the Mongo implementation of the deal-timeline repository.
// Every operation is scoped by the tenant from the context.
package dealtimeline

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/dealtimeline/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"

	"go.mongodb.org/mongo-driver/bson"
)

// Repository implements repository.TimelineRepository over MongoDB.
type Repository struct {
	coll *mongo.Collection
}

// New builds the repository.
func New(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("deal_timeline")}
}

func (r *Repository) Append(ctx context.Context, ev *entity.Event) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toModel(ev))
	return mongodb.MapError(err)
}

func (r *Repository) ListByDeal(ctx context.Context, dealID string, page shared.PageRequest) ([]*entity.Event, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	base := bson.M{"tenant_id": tenantID, "deal_id": dealID}
	filter := mongodb.ApplyKeyset(base, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.Event
	for c.Next(ctx) {
		var m models.DealTimelineEvent
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toModel(ev *entity.Event) models.DealTimelineEvent {
	return models.DealTimelineEvent{
		ID: ev.ID, TenantID: ev.TenantID, DealID: ev.DealID, Kind: string(ev.Kind),
		ActorID: ev.ActorID, Data: ev.Data, CreatedAt: ev.CreatedAt,
	}
}

func toEntity(m *models.DealTimelineEvent) *entity.Event {
	return &entity.Event{
		ID: m.ID, TenantID: m.TenantID, DealID: m.DealID, Kind: entity.Kind(m.Kind),
		ActorID: m.ActorID, Data: m.Data, CreatedAt: m.CreatedAt,
	}
}

var _ repository.TimelineRepository = (*Repository)(nil)
