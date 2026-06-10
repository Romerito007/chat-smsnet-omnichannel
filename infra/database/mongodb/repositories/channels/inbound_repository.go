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

// InboundRepository implements repository.InboundRepository (idempotency ledger).
// A unique index on (tenant_id, channel, external_message_id) is the backstop
// against duplicate processing.
type InboundRepository struct {
	coll *mongo.Collection
}

// NewInboundRepository builds the repository.
func NewInboundRepository(db *mongo.Database) *InboundRepository {
	return &InboundRepository{coll: db.Collection("inbound_messages")}
}

func (r *InboundRepository) FindByExternalID(ctx context.Context, channel, externalMessageID string) (*entity.InboundRecord, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.InboundRecord
	if err := r.coll.FindOne(ctx, bson.M{
		"tenant_id":           tenantID,
		"channel":             channel,
		"external_message_id": externalMessageID,
	}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *InboundRepository) Create(ctx context.Context, rec *entity.InboundRecord) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, models.InboundRecord{
		ID:                rec.ID,
		TenantID:          rec.TenantID,
		Channel:           rec.Channel,
		ExternalMessageID: rec.ExternalMessageID,
		ConversationID:    rec.ConversationID,
		MessageID:         rec.MessageID,
		CreatedAt:         rec.CreatedAt,
	})
	return mongodb.MapError(err)
}

func toEntity(m *models.InboundRecord) *entity.InboundRecord {
	return &entity.InboundRecord{
		ID:                m.ID,
		TenantID:          m.TenantID,
		Channel:           m.Channel,
		ExternalMessageID: m.ExternalMessageID,
		ConversationID:    m.ConversationID,
		MessageID:         m.MessageID,
		CreatedAt:         m.CreatedAt,
	}
}

var _ repository.InboundRepository = (*InboundRepository)(nil)
