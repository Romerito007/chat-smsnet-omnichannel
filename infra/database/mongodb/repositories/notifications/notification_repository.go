// Package notifications is the Mongo implementation of the notifications
// repositories.
package notifications

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// NotificationRepository implements repository.NotificationRepository.
type NotificationRepository struct {
	coll *mongo.Collection
}

// NewNotificationRepository builds the repository.
func NewNotificationRepository(db *mongo.Database) *NotificationRepository {
	return &NotificationRepository{coll: db.Collection("notifications")}
}

func (r *NotificationRepository) Create(ctx context.Context, n *entity.Notification) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toModel(n))
	return mongodb.MapError(err)
}

func (r *NotificationRepository) FindByID(ctx context.Context, id string) (*entity.Notification, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Notification
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *NotificationRepository) ListByUser(ctx context.Context, userID string, unreadOnly bool, page shared.PageRequest) ([]*entity.Notification, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	base := bson.M{"tenant_id": tenantID, "user_id": userID}
	if unreadOnly {
		base["read"] = false
	}
	filter := mongodb.ApplyKeyset(base, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer c.Close(ctx)
	var out []*entity.Notification
	for c.Next(ctx) {
		var m models.Notification
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func (r *NotificationRepository) MarkRead(ctx context.Context, id, userID string, at time.Time) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": id, "tenant_id": tenantID, "user_id": userID},
		bson.M{"$set": bson.M{"read": true, "read_at": at}},
	)
	if err != nil {
		return mongodb.MapError(err)
	}
	if res.MatchedCount == 0 {
		return mongodb.MapError(mongo.ErrNoDocuments)
	}
	return nil
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID string, at time.Time) (int, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, err
	}
	res, err := r.coll.UpdateMany(ctx,
		bson.M{"tenant_id": tenantID, "user_id": userID, "read": false},
		bson.M{"$set": bson.M{"read": true, "read_at": at}},
	)
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	return int(res.ModifiedCount), nil
}

func (r *NotificationRepository) DeleteReadBefore(ctx context.Context, before time.Time) (int, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, err
	}
	res, err := r.coll.DeleteMany(ctx, bson.M{
		"tenant_id":  tenantID,
		"read":       true,
		"created_at": bson.M{"$lte": before},
	})
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	return int(res.DeletedCount), nil
}

func toModel(n *entity.Notification) models.Notification {
	return models.Notification{
		ID: n.ID, TenantID: n.TenantID, UserID: n.UserID, Type: string(n.Type),
		Title: n.Title, Body: n.Body, Link: n.Link, Read: n.Read,
		CreatedAt: n.CreatedAt, ReadAt: n.ReadAt,
	}
}

func toEntity(m *models.Notification) *entity.Notification {
	return &entity.Notification{
		ID: m.ID, TenantID: m.TenantID, UserID: m.UserID, Type: entity.Type(m.Type),
		Title: m.Title, Body: m.Body, Link: m.Link, Read: m.Read,
		CreatedAt: m.CreatedAt, ReadAt: m.ReadAt,
	}
}

var _ repository.NotificationRepository = (*NotificationRepository)(nil)
