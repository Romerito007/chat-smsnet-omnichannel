// Package contacts is the Mongo implementation of the contact repository.
package contacts

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/contacts/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// Repository implements repository.ContactRepository over MongoDB.
type Repository struct {
	coll *mongo.Collection
}

// New builds the repository.
func New(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("contacts")}
}

func (r *Repository) Create(ctx context.Context, c *entity.Contact) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toModel(c))
	return mongodb.MapError(err)
}

func (r *Repository) Update(ctx context.Context, c *entity.Contact) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": c.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":       c.Name,
			"phone":      c.Phone,
			"document":   c.Document,
			"identities": toIdentityModels(c.Identities),
			"updated_at": c.UpdatedAt,
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

func (r *Repository) FindByID(ctx context.Context, id string) (*entity.Contact, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Contact
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *Repository) FindByChannelIdentity(ctx context.Context, channel, externalID string) (*entity.Contact, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	filter := bson.M{
		"tenant_id": tenantID,
		"identities": bson.M{"$elemMatch": bson.M{
			"channel":     channel,
			"external_id": externalID,
		}},
	}
	var m models.Contact
	if err := r.coll.FindOne(ctx, filter).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *Repository) List(ctx context.Context, page shared.PageRequest) ([]*entity.Contact, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	filter := mongodb.ApplyKeyset(bson.M{"tenant_id": tenantID}, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer c.Close(ctx)
	var out []*entity.Contact
	for c.Next(ctx) {
		var m models.Contact
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toModel(c *entity.Contact) models.Contact {
	m := models.Contact{
		Name:       c.Name,
		Phone:      c.Phone,
		Document:   c.Document,
		Identities: toIdentityModels(c.Identities),
	}
	m.ID = c.ID
	m.TenantID = c.TenantID
	m.CreatedAt = c.CreatedAt
	m.UpdatedAt = c.UpdatedAt
	return m
}

func toEntity(m *models.Contact) *entity.Contact {
	ids := make([]entity.ChannelIdentity, len(m.Identities))
	for i, id := range m.Identities {
		ids[i] = entity.ChannelIdentity{Channel: id.Channel, ExternalID: id.ExternalID}
	}
	return &entity.Contact{
		ID:         m.ID,
		TenantID:   m.TenantID,
		Name:       m.Name,
		Phone:      m.Phone,
		Document:   m.Document,
		Identities: ids,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}

func toIdentityModels(ids []entity.ChannelIdentity) []models.ChannelIdentity {
	out := make([]models.ChannelIdentity, len(ids))
	for i, id := range ids {
		out[i] = models.ChannelIdentity{Channel: id.Channel, ExternalID: id.ExternalID}
	}
	return out
}

var _ repository.ContactRepository = (*Repository)(nil)
