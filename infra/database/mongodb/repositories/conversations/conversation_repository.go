// Package conversations is the Mongo implementation of the conversations
// repositories (conversations, messages, events). Every operation is scoped by
// the tenant from the context.
package conversations

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// ConversationRepository implements repository.ConversationRepository.
type ConversationRepository struct {
	coll *mongo.Collection
}

// NewConversationRepository builds the repository.
func NewConversationRepository(db *mongo.Database) *ConversationRepository {
	return &ConversationRepository{coll: db.Collection("conversations")}
}

func (r *ConversationRepository) Create(ctx context.Context, c *entity.Conversation) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, convToModel(c))
	return mongodb.MapError(err)
}

func (r *ConversationRepository) Update(ctx context.Context, c *entity.Conversation) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": c.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"sector_id":       c.SectorID,
			"queue_id":        c.QueueID,
			"status":          string(c.Status),
			"assigned_to":     c.AssignedTo,
			"priority":        string(c.Priority),
			"tags":            c.Tags,
			"last_message_at": c.LastMessageAt,
			"unread_count":    c.UnreadCount,
			"last_read_at":    c.LastReadAt,
			"updated_at":      c.UpdatedAt,
			"closed_at":       c.ClosedAt,
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

func (r *ConversationRepository) FindByID(ctx context.Context, id string) (*entity.Conversation, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Conversation
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return convToEntity(&m), nil
}

func (r *ConversationRepository) FindByIDs(ctx context.Context, ids []string) ([]*entity.Conversation, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	cur, err := r.coll.Find(ctx, bson.M{"_id": bson.M{"$in": ids}, "tenant_id": tenantID})
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []*entity.Conversation
	for cur.Next(ctx) {
		var m models.Conversation
		if err := cur.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, convToEntity(&m))
	}
	return out, mongodb.MapError(cur.Err())
}

// closedStatuses are the terminal conversation states excluded from "open".
var closedStatuses = bson.A{
	string(entity.StatusResolved), string(entity.StatusClosed), string(entity.StatusArchived),
}

func (r *ConversationRepository) FindOpenByContactChannelID(ctx context.Context, contactID, channelID string) (*entity.Conversation, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	filter := bson.M{
		"tenant_id":  tenantID,
		"contact_id": contactID,
		"channel_id": channelID,
		"status":     bson.M{"$nin": closedStatuses},
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "updated_at", Value: -1}})
	var m models.Conversation
	if err := r.coll.FindOne(ctx, filter, opts).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return convToEntity(&m), nil
}

// FindLastByContactChannelID returns the most recent conversation for a contact
// on a specific channel connection REGARDLESS of status (open or closed), or a
// not_found AppError. Used by single-mode inbound to reopen the last conversation.
func (r *ConversationRepository) FindLastByContactChannelID(ctx context.Context, contactID, channelID string) (*entity.Conversation, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	filter := bson.M{
		"tenant_id":  tenantID,
		"contact_id": contactID,
		"channel_id": channelID,
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "updated_at", Value: -1}})
	var m models.Conversation
	if err := r.coll.FindOne(ctx, filter, opts).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return convToEntity(&m), nil
}

func (r *ConversationRepository) ListInactiveOpen(ctx context.Context, idleBefore time.Time, limit int) ([]*entity.Conversation, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 500
	}
	filter := bson.M{
		"tenant_id":       tenantID,
		"status":          bson.M{"$nin": closedStatuses},
		"last_message_at": bson.M{"$lte": idleBefore},
	}
	opts := options.Find().SetSort(bson.D{{Key: "last_message_at", Value: 1}}).SetLimit(int64(limit))
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.Conversation
	for c.Next(ctx) {
		var m models.Conversation
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, convToEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func (r *ConversationRepository) List(ctx context.Context, filter contracts.ListFilter, vis contracts.Visibility, page shared.PageRequest) ([]*entity.Conversation, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}

	base := bson.M{"tenant_id": tenantID}
	if filter.Status != "" {
		base["status"] = filter.Status
	}
	if filter.SectorID != "" {
		base["sector_id"] = filter.SectorID
	}
	if filter.QueueID != "" {
		base["queue_id"] = filter.QueueID
	}
	if filter.AssignedTo != "" {
		base["assigned_to"] = filter.AssignedTo
	}
	if filter.ContactID != "" {
		base["contact_id"] = filter.ContactID
	}
	if filter.Protocol != "" {
		base["protocol"] = filter.Protocol
	}
	if filter.Tag != "" {
		base["tags"] = filter.Tag
	}

	// Visibility: when not all-scope, restrict to assigned-to-me OR my sectors.
	if !vis.All {
		base["$or"] = bson.A{
			bson.M{"assigned_to": vis.UserID},
			bson.M{"sector_id": bson.M{"$in": nonEmpty(vis.SectorIDs)}},
		}
	}

	full := mongodb.ApplyKeysetField(base, cur, "updated_at")
	opts := options.Find().SetSort(mongodb.KeysetSortField("updated_at")).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, full, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.Conversation
	for c.Next(ctx) {
		var m models.Conversation
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, convToEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

// nonEmpty guarantees a non-nil slice so $in never matches everything by accident.
func nonEmpty(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

func convToModel(c *entity.Conversation) models.Conversation {
	m := models.Conversation{
		ContactID:     c.ContactID,
		Channel:       c.Channel,
		ChannelID:     c.ChannelID,
		SectorID:      c.SectorID,
		QueueID:       c.QueueID,
		Status:        string(c.Status),
		AssignedTo:    c.AssignedTo,
		Priority:      string(c.Priority),
		Protocol:      c.Protocol,
		Tags:          c.Tags,
		LastMessageAt: c.LastMessageAt,
		UnreadCount:   c.UnreadCount,
		LastReadAt:    c.LastReadAt,
		ClosedAt:      c.ClosedAt,
	}
	m.ID = c.ID
	m.TenantID = c.TenantID
	m.CreatedAt = c.CreatedAt
	m.UpdatedAt = c.UpdatedAt
	return m
}

func convToEntity(m *models.Conversation) *entity.Conversation {
	return &entity.Conversation{
		ID:            m.ID,
		TenantID:      m.TenantID,
		ContactID:     m.ContactID,
		Channel:       m.Channel,
		ChannelID:     m.ChannelID,
		SectorID:      m.SectorID,
		QueueID:       m.QueueID,
		Status:        entity.Status(m.Status),
		AssignedTo:    m.AssignedTo,
		Priority:      entity.Priority(m.Priority),
		Protocol:      m.Protocol,
		Tags:          m.Tags,
		LastMessageAt: m.LastMessageAt,
		UnreadCount:   m.UnreadCount,
		LastReadAt:    m.LastReadAt,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
		ClosedAt:      m.ClosedAt,
	}
}

var _ repository.ConversationRepository = (*ConversationRepository)(nil)
