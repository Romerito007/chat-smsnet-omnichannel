package webhooks

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// DeliveryRepository implements repository.DeliveryRepository.
type DeliveryRepository struct {
	coll *mongo.Collection
}

// NewDeliveryRepository builds the repository.
func NewDeliveryRepository(db *mongo.Database) *DeliveryRepository {
	return &DeliveryRepository{coll: db.Collection("webhook_deliveries")}
}

func (r *DeliveryRepository) Create(ctx context.Context, d *entity.WebhookDelivery) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toDeliveryModel(d))
	return mongodb.MapError(err)
}

func (r *DeliveryRepository) Update(ctx context.Context, d *entity.WebhookDelivery) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": d.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"status":        string(d.Status),
			"attempts":      d.Attempts,
			"last_error":    d.LastError,
			"next_retry_at": d.NextRetryAt,
			"updated_at":    d.UpdatedAt,
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

func (r *DeliveryRepository) FindByID(ctx context.Context, id string) (*entity.WebhookDelivery, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.WebhookDelivery
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toDeliveryEntity(&m), nil
}

func (r *DeliveryRepository) ListByWebhook(ctx context.Context, webhookID string, page shared.PageRequest) ([]*entity.WebhookDelivery, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	filter := mongodb.ApplyKeyset(bson.M{"tenant_id": tenantID, "webhook_id": webhookID}, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.WebhookDelivery
	for c.Next(ctx) {
		var m models.WebhookDelivery
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toDeliveryEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toDeliveryModel(d *entity.WebhookDelivery) models.WebhookDelivery {
	return models.WebhookDelivery{
		ID:          d.ID,
		TenantID:    d.TenantID,
		WebhookID:   d.WebhookID,
		Event:       d.Event,
		Payload:     d.Payload,
		Status:      string(d.Status),
		Attempts:    d.Attempts,
		LastError:   d.LastError,
		NextRetryAt: d.NextRetryAt,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func toDeliveryEntity(m *models.WebhookDelivery) *entity.WebhookDelivery {
	return &entity.WebhookDelivery{
		ID:          m.ID,
		TenantID:    m.TenantID,
		WebhookID:   m.WebhookID,
		Event:       m.Event,
		Payload:     m.Payload,
		Status:      entity.DeliveryStatus(m.Status),
		Attempts:    m.Attempts,
		LastError:   m.LastError,
		NextRetryAt: m.NextRetryAt,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

var _ repository.DeliveryRepository = (*DeliveryRepository)(nil)
