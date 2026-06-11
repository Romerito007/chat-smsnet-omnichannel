// Package search is the Mongo-backed search Index: it queries the conversations,
// contacts, messages (text index) and sla_trackings collections. Swapping to a
// dedicated engine means a new Index implementation; the domain is untouched.
package search

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	contactentity "github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	conventity "github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/search/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/search/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// Index implements repository.Index over Mongo.
type Index struct {
	conversations *mongo.Collection
	contacts      *mongo.Collection
	messages      *mongo.Collection
	slaTrackings  *mongo.Collection
}

// NewIndex builds the index.
func NewIndex(db *mongo.Database) *Index {
	return &Index{
		conversations: db.Collection("conversations"),
		contacts:      db.Collection("contacts"),
		messages:      db.Collection("messages"),
		slaTrackings:  db.Collection("sla_trackings"),
	}
}

func (i *Index) SearchConversations(ctx context.Context, f contracts.ConversationFilter, vis contracts.Visibility, page shared.PageRequest) ([]*conventity.Conversation, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}

	base := bson.M{"tenant_id": tenantID}
	if f.Status != "" {
		base["status"] = f.Status
	}
	if f.SectorID != "" {
		base["sector_id"] = f.SectorID
	}
	if f.AssignedTo != "" {
		base["assigned_to"] = f.AssignedTo
	}
	if f.Channel != "" {
		base["channel"] = f.Channel
	}
	if f.Tag != "" {
		base["tags"] = f.Tag
	}
	if f.Priority != "" {
		base["priority"] = f.Priority
	}
	if f.From != nil || f.To != nil {
		rng := bson.M{}
		if f.From != nil {
			rng["$gte"] = *f.From
		}
		if f.To != nil {
			rng["$lte"] = *f.To
		}
		base["created_at"] = rng
	}
	if f.SLAStatus != "" {
		ids, err := i.conversationIDsBySLA(ctx, tenantID, f.SLAStatus)
		if err != nil {
			return nil, err
		}
		base["_id"] = bson.M{"$in": ids}
	}

	// Visibility: non-all actors see assigned-to-me OR in-my-sectors.
	if !vis.All {
		base["$or"] = bson.A{
			bson.M{"assigned_to": vis.UserID},
			bson.M{"sector_id": bson.M{"$in": nonEmpty(vis.SectorIDs)}},
		}
	}

	full := mongodb.ApplyKeysetField(base, cur, "updated_at")
	opts := options.Find().SetSort(mongodb.KeysetSortField("updated_at")).SetLimit(int64(page.Limit) + 1)
	c, err := i.conversations.Find(ctx, full, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*conventity.Conversation
	for c.Next(ctx) {
		var m models.Conversation
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toConversation(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func (i *Index) SearchContactsText(ctx context.Context, query string, cur shared.Cursor, scanLimit int) ([]*contactentity.Contact, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	base := bson.M{"tenant_id": tenantID}
	if q := normalize(query); q != "" {
		base["$text"] = bson.M{"$search": q}
	}
	filter := mongodb.ApplyKeyset(base, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(scanLimit))
	c, err := i.contacts.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*contactentity.Contact
	for c.Next(ctx) {
		var m models.Contact
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toContact(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func (i *Index) SearchMessagesText(ctx context.Context, query, conversationID string, cur shared.Cursor, scanLimit int) ([]*conventity.Message, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	base := bson.M{"tenant_id": tenantID, "deleted_at": bson.M{"$exists": false}}
	if conversationID != "" {
		base["conversation_id"] = conversationID
	}
	if q := normalize(query); q != "" {
		base["$text"] = bson.M{"$search": q}
	}
	filter := mongodb.ApplyKeyset(base, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(scanLimit))
	c, err := i.messages.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*conventity.Message
	for c.Next(ctx) {
		var m models.Message
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toMessage(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func (i *Index) FindConversation(ctx context.Context, id string) (*conventity.Conversation, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Conversation
	if err := i.conversations.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toConversation(&m), nil
}

func (i *Index) HasVisibleConversationForContact(ctx context.Context, contactID string, vis contracts.Visibility) (bool, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return false, err
	}
	filter := bson.M{"tenant_id": tenantID, "contact_id": contactID}
	if !vis.All {
		filter["$or"] = bson.A{
			bson.M{"assigned_to": vis.UserID},
			bson.M{"sector_id": bson.M{"$in": nonEmpty(vis.SectorIDs)}},
		}
	}
	n, err := i.conversations.CountDocuments(ctx, filter, options.Count().SetLimit(1))
	if err != nil {
		return false, mongodb.MapError(err)
	}
	return n > 0, nil
}

// conversationIDsBySLA returns conversation ids whose SLA tracking matches the
// requested coarse status.
func (i *Index) conversationIDsBySLA(ctx context.Context, tenantID, status string) ([]string, error) {
	filter := bson.M{"tenant_id": tenantID}
	switch status {
	case "breached":
		filter["$or"] = bson.A{bson.M{"first_response_breached": true}, bson.M{"resolution_breached": true}}
	case "at_risk":
		filter["$and"] = bson.A{
			bson.M{"$or": bson.A{bson.M{"first_response_warned": true}, bson.M{"resolution_warned": true}}},
			bson.M{"first_response_breached": false},
			bson.M{"resolution_breached": false},
		}
	case "met":
		filter["status"] = "met"
	case "running":
		filter["status"] = "running"
	default:
		return []string{}, nil
	}
	cur, err := i.slaTrackings.Find(ctx, filter, options.Find().SetProjection(bson.M{"conversation_id": 1}))
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	ids := []string{}
	for cur.Next(ctx) {
		var doc struct {
			ConversationID string `bson:"conversation_id"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, mongodb.MapError(err)
		}
		if doc.ConversationID != "" {
			ids = append(ids, doc.ConversationID)
		}
	}
	return ids, mongodb.MapError(cur.Err())
}

func nonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return []string{"\x00"} // never matches
	}
	return out
}

var _ repository.Index = (*Index)(nil)
