package channels

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// OutboundDeliveryRepository implements repository.OutboundDeliveryRepository.
type OutboundDeliveryRepository struct {
	coll *mongo.Collection
}

// NewOutboundDeliveryRepository builds the repository.
func NewOutboundDeliveryRepository(db *mongo.Database) *OutboundDeliveryRepository {
	return &OutboundDeliveryRepository{coll: db.Collection("outbound_deliveries")}
}

func (r *OutboundDeliveryRepository) Create(ctx context.Context, d *entity.OutboundDelivery) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, deliveryToModel(d))
	return mongodb.MapError(err)
}

func (r *OutboundDeliveryRepository) Update(ctx context.Context, d *entity.OutboundDelivery) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": d.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"status":              string(d.Status),
			"attempts":            d.Attempts,
			"external_message_id": d.ExternalMessageID,
			"last_error":          d.LastError,
			"next_retry_at":       d.NextRetryAt,
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

func (r *OutboundDeliveryRepository) FindByID(ctx context.Context, id string) (*entity.OutboundDelivery, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.OutboundDelivery
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return deliveryToEntity(&m), nil
}

func (r *OutboundDeliveryRepository) FindByExternalMessageID(ctx context.Context, externalMessageID string) (*entity.OutboundDelivery, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.OutboundDelivery
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "external_message_id": externalMessageID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return deliveryToEntity(&m), nil
}

func deliveryToModel(d *entity.OutboundDelivery) models.OutboundDelivery {
	return models.OutboundDelivery{
		ID:                  d.ID,
		TenantID:            d.TenantID,
		ChannelConnectionID: d.ChannelConnectionID,
		ConversationID:      d.ConversationID,
		MessageID:           d.MessageID,
		Status:              string(d.Status),
		Attempts:            d.Attempts,
		ExternalMessageID:   d.ExternalMessageID,
		LastError:           d.LastError,
		NextRetryAt:         d.NextRetryAt,
		CreatedAt:           d.CreatedAt,
		UpdatedAt:           d.UpdatedAt,
	}
}

func deliveryToEntity(m *models.OutboundDelivery) *entity.OutboundDelivery {
	return &entity.OutboundDelivery{
		ID:                  m.ID,
		TenantID:            m.TenantID,
		ChannelConnectionID: m.ChannelConnectionID,
		ConversationID:      m.ConversationID,
		MessageID:           m.MessageID,
		Status:              entity.DeliveryStatus(m.Status),
		Attempts:            m.Attempts,
		ExternalMessageID:   m.ExternalMessageID,
		LastError:           m.LastError,
		NextRetryAt:         m.NextRetryAt,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
	}
}

var _ repository.OutboundDeliveryRepository = (*OutboundDeliveryRepository)(nil)
