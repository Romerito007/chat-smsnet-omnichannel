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

// MessageRepository implements repository.MessageRepository.
type MessageRepository struct {
	coll *mongo.Collection
}

// NewMessageRepository builds the repository.
func NewMessageRepository(db *mongo.Database) *MessageRepository {
	return &MessageRepository{coll: db.Collection("messages")}
}

func (r *MessageRepository) Create(ctx context.Context, m *entity.Message) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, msgToModel(m))
	return mongodb.MapError(err)
}

func (r *MessageRepository) Update(ctx context.Context, m *entity.Message) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": m.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"text":                m.Text,
			"delivery_status":     string(m.DeliveryStatus),
			"delivery_error":      m.DeliveryError,
			"external_message_id": m.ExternalMessageID,
			"delivered_at":        m.DeliveredAt,
			"read_at":             m.ReadAt,
			"edited_at":           m.EditedAt,
			"deleted_at":          m.DeletedAt,
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

func (r *MessageRepository) FindByID(ctx context.Context, id string) (*entity.Message, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Message
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return msgToEntity(&m), nil
}

func (r *MessageRepository) ListByConversation(ctx context.Context, conversationID string, page shared.PageRequest) ([]*entity.Message, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	base := bson.M{
		"tenant_id":       tenantID,
		"conversation_id": conversationID,
		"deleted_at":      bson.M{"$eq": nil}, // soft-deleted messages are hidden
	}
	full := mongodb.ApplyKeyset(base, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, full, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.Message
	for c.Next(ctx) {
		var m models.Message
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, msgToEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func msgToModel(m *entity.Message) models.Message {
	atts := make([]models.Attachment, len(m.Attachments))
	for i, a := range m.Attachments {
		atts[i] = models.Attachment{ID: a.ID, URL: a.URL, ContentType: a.ContentType, Filename: a.Filename, Size: a.Size}
	}
	return models.Message{
		ID:                m.ID,
		TenantID:          m.TenantID,
		ConversationID:    m.ConversationID,
		SenderType:        string(m.SenderType),
		SenderID:          m.SenderID,
		Direction:         string(m.Direction),
		MessageType:       string(m.MessageType),
		Text:              m.Text,
		Attachments:       atts,
		Metadata:          m.Metadata,
		CreatedAt:         m.CreatedAt,
		DeliveryStatus:    string(m.DeliveryStatus),
		DeliveryError:     m.DeliveryError,
		ExternalMessageID: m.ExternalMessageID,
		DeliveredAt:       m.DeliveredAt,
		ReadAt:            m.ReadAt,
		EditedAt:          m.EditedAt,
		DeletedAt:         m.DeletedAt,
	}
}

func msgToEntity(m *models.Message) *entity.Message {
	atts := make([]entity.Attachment, len(m.Attachments))
	for i, a := range m.Attachments {
		atts[i] = entity.Attachment{ID: a.ID, URL: a.URL, ContentType: a.ContentType, Filename: a.Filename, Size: a.Size}
	}
	return &entity.Message{
		ID:                m.ID,
		TenantID:          m.TenantID,
		ConversationID:    m.ConversationID,
		SenderType:        entity.SenderType(m.SenderType),
		SenderID:          m.SenderID,
		Direction:         entity.Direction(m.Direction),
		MessageType:       entity.MessageType(m.MessageType),
		Text:              m.Text,
		Attachments:       atts,
		Metadata:          m.Metadata,
		CreatedAt:         m.CreatedAt,
		DeliveryStatus:    entity.DeliveryStatus(m.DeliveryStatus),
		DeliveryError:     m.DeliveryError,
		ExternalMessageID: m.ExternalMessageID,
		DeliveredAt:       m.DeliveredAt,
		ReadAt:            m.ReadAt,
		EditedAt:          m.EditedAt,
		DeletedAt:         m.DeletedAt,
	}
}

var _ repository.MessageRepository = (*MessageRepository)(nil)
