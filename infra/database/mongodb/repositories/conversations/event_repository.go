package conversations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// EventRepository implements repository.EventRepository.
type EventRepository struct {
	coll *mongo.Collection
}

// NewEventRepository builds the repository.
func NewEventRepository(db *mongo.Database) *EventRepository {
	return &EventRepository{coll: db.Collection("conversation_events")}
}

func (r *EventRepository) Create(ctx context.Context, e *entity.ConversationEvent) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, eventToModel(e))
	return mongodb.MapError(err)
}

func (r *EventRepository) ListByConversation(ctx context.Context, conversationID string, page shared.PageRequest) ([]*entity.ConversationEvent, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	base := bson.M{"tenant_id": tenantID, "conversation_id": conversationID}
	full := mongodb.ApplyKeyset(base, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, full, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer c.Close(ctx)
	var out []*entity.ConversationEvent
	for c.Next(ctx) {
		var m models.ConversationEvent
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, eventToEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func eventToModel(e *entity.ConversationEvent) models.ConversationEvent {
	return models.ConversationEvent{
		ID:             e.ID,
		TenantID:       e.TenantID,
		ConversationID: e.ConversationID,
		Type:           e.Type,
		ActorType:      string(e.ActorType),
		ActorID:        e.ActorID,
		Data:           e.Data,
		CreatedAt:      e.CreatedAt,
	}
}

func eventToEntity(m *models.ConversationEvent) *entity.ConversationEvent {
	return &entity.ConversationEvent{
		ID:             m.ID,
		TenantID:       m.TenantID,
		ConversationID: m.ConversationID,
		Type:           m.Type,
		ActorType:      entity.ActorType(m.ActorType),
		ActorID:        m.ActorID,
		Data:           m.Data,
		CreatedAt:      m.CreatedAt,
	}
}

var _ repository.EventRepository = (*EventRepository)(nil)
