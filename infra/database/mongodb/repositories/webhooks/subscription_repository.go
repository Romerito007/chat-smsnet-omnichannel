// Package webhooks is the Mongo implementation of the webhooks repositories
// (subscriptions and per-attempt deliveries).
package webhooks

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/webhooks/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/secrets"
)

// SubscriptionRepository implements repository.SubscriptionRepository. The
// signing secret is encrypted at rest.
type SubscriptionRepository struct {
	coll   *mongo.Collection
	cipher *secrets.Cipher
}

// NewSubscriptionRepository builds the repository.
func NewSubscriptionRepository(db *mongo.Database, cipher *secrets.Cipher) *SubscriptionRepository {
	return &SubscriptionRepository{coll: db.Collection("webhook_subscriptions"), cipher: cipher}
}

func (r *SubscriptionRepository) Create(ctx context.Context, s *entity.WebhookSubscription) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	enc, err := r.cipher.Encrypt(s.Secret)
	if err != nil {
		return apperror.Internal("encrypt secret").Wrap(err)
	}
	_, err = r.coll.InsertOne(ctx, toModel(s, enc))
	return mongodb.MapError(err)
}

func (r *SubscriptionRepository) Update(ctx context.Context, s *entity.WebhookSubscription) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	enc, err := r.cipher.Encrypt(s.Secret)
	if err != nil {
		return apperror.Internal("encrypt secret").Wrap(err)
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": s.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":                  s.Name,
			"url":                   s.URL,
			"events":                s.Events,
			"scopes":                s.Scopes,
			"encrypted_secret":      enc,
			"enabled":               s.Enabled,
			"rate_limit_per_minute": s.RateLimitPerMin,
			"owned_by_channel_id":   s.OwnedByChannelID,
			"updated_at":            s.UpdatedAt,
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

func (r *SubscriptionRepository) Delete(ctx context.Context, id string) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id, "tenant_id": tenantID})
	if err != nil {
		return mongodb.MapError(err)
	}
	if res.DeletedCount == 0 {
		return mongodb.MapError(mongo.ErrNoDocuments)
	}
	return nil
}

func (r *SubscriptionRepository) FindByID(ctx context.Context, id string) (*entity.WebhookSubscription, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.WebhookSubscription
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return r.toEntity(&m)
}

func (r *SubscriptionRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.WebhookSubscription, error) {
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
	defer func() { _ = c.Close(ctx) }()
	return r.decodeAll(ctx, c)
}

func (r *SubscriptionRepository) ListEnabledByEvent(ctx context.Context, tenantID, event string) ([]*entity.WebhookSubscription, error) {
	c, err := r.coll.Find(ctx, bson.M{"tenant_id": tenantID, "enabled": true, "events": event})
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	return r.decodeAll(ctx, c)
}

// FindByChannelID returns the subscription managed by the given channel
// connection, or a not-found error when the channel has none.
func (r *SubscriptionRepository) FindByChannelID(ctx context.Context, channelID string) (*entity.WebhookSubscription, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.WebhookSubscription
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "owned_by_channel_id": channelID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return r.toEntity(&m)
}

func (r *SubscriptionRepository) decodeAll(ctx context.Context, c *mongo.Cursor) ([]*entity.WebhookSubscription, error) {
	var out []*entity.WebhookSubscription
	for c.Next(ctx) {
		var m models.WebhookSubscription
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		e, err := r.toEntity(&m)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, mongodb.MapError(c.Err())
}

func toModel(s *entity.WebhookSubscription, encryptedSecret string) models.WebhookSubscription {
	m := models.WebhookSubscription{
		Name:             s.Name,
		URL:              s.URL,
		Events:           s.Events,
		Scopes:           s.Scopes,
		EncryptedSecret:  encryptedSecret,
		Enabled:          s.Enabled,
		RateLimitPerMin:  s.RateLimitPerMin,
		OwnedByChannelID: s.OwnedByChannelID,
		CreatedBy:        s.CreatedBy,
	}
	m.ID = s.ID
	m.TenantID = s.TenantID
	m.CreatedAt = s.CreatedAt
	m.UpdatedAt = s.UpdatedAt
	return m
}

func (r *SubscriptionRepository) toEntity(m *models.WebhookSubscription) (*entity.WebhookSubscription, error) {
	secret, err := r.cipher.Decrypt(m.EncryptedSecret)
	if err != nil {
		return nil, apperror.Internal("decrypt secret").Wrap(err)
	}
	return &entity.WebhookSubscription{
		ID:               m.ID,
		TenantID:         m.TenantID,
		Name:             m.Name,
		URL:              m.URL,
		Events:           m.Events,
		Scopes:           m.Scopes,
		Secret:           secret,
		Enabled:          m.Enabled,
		RateLimitPerMin:  m.RateLimitPerMin,
		OwnedByChannelID: m.OwnedByChannelID,
		CreatedBy:        m.CreatedBy,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}, nil
}

var _ repository.SubscriptionRepository = (*SubscriptionRepository)(nil)
