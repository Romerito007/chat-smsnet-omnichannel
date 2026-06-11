// Package channels is the Mongo implementation of the channels repositories
// (connections, outbound deliveries and the inbound idempotency ledger).
package channels

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/secrets"
)

// ConnectionRepository implements repository.ConnectionRepository. The secret is
// encrypted on write and decrypted on read so plaintext is never persisted.
type ConnectionRepository struct {
	coll   *mongo.Collection
	cipher *secrets.Cipher
}

// NewConnectionRepository builds the repository.
func NewConnectionRepository(db *mongo.Database, cipher *secrets.Cipher) *ConnectionRepository {
	return &ConnectionRepository{coll: db.Collection("channel_connections"), cipher: cipher}
}

func (r *ConnectionRepository) Create(ctx context.Context, c *entity.ChannelConnection) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	m, err := r.toModel(c)
	if err != nil {
		return apperror.Internal("encrypt secret").Wrap(err)
	}
	_, err = r.coll.InsertOne(ctx, m)
	return mongodb.MapError(err)
}

func (r *ConnectionRepository) Update(ctx context.Context, c *entity.ChannelConnection) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	enc, err := r.cipher.Encrypt(c.Secret)
	if err != nil {
		return apperror.Internal("encrypt secret").Wrap(err)
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": c.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":               c.Name,
			"status":             string(c.Status),
			"base_url":           c.BaseURL,
			"auth_type":          string(c.AuthType),
			"encrypted_secret":   enc,
			"default_sector_id":  c.DefaultSectorID,
			"enabled":            c.Enabled,
			"automation_enabled": c.AutomationEnabled,
			"updated_at":         c.UpdatedAt,
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

func (r *ConnectionRepository) Delete(ctx context.Context, id string) error {
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

func (r *ConnectionRepository) FindByID(ctx context.Context, id string) (*entity.ChannelConnection, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.ChannelConnection
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return r.toEntity(&m)
}

func (r *ConnectionRepository) FindEnabledByType(ctx context.Context, t entity.Type) (*entity.ChannelConnection, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.ChannelConnection
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "type": string(t), "enabled": true}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return r.toEntity(&m)
}

// FindByWebhookVerifyToken is not tenant-scoped (pre-auth inbound/receipts).
func (r *ConnectionRepository) FindByWebhookVerifyToken(ctx context.Context, token string) (*entity.ChannelConnection, error) {
	var m models.ChannelConnection
	if err := r.coll.FindOne(ctx, bson.M{"webhook_verify_token": token}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return r.toEntity(&m)
}

func (r *ConnectionRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.ChannelConnection, error) {
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
	var out []*entity.ChannelConnection
	for c.Next(ctx) {
		var m models.ChannelConnection
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

func (r *ConnectionRepository) toModel(c *entity.ChannelConnection) (models.ChannelConnection, error) {
	enc, err := r.cipher.Encrypt(c.Secret)
	if err != nil {
		return models.ChannelConnection{}, err
	}
	m := models.ChannelConnection{
		Type:               string(c.Type),
		Name:               c.Name,
		Status:             string(c.Status),
		BaseURL:            c.BaseURL,
		AuthType:           string(c.AuthType),
		EncryptedSecret:    enc,
		WebhookVerifyToken: c.WebhookVerifyToken,
		DefaultSectorID:    c.DefaultSectorID,
		Enabled:            c.Enabled,
		AutomationEnabled:  c.AutomationEnabled,
	}
	m.ID = c.ID
	m.TenantID = c.TenantID
	m.CreatedAt = c.CreatedAt
	m.UpdatedAt = c.UpdatedAt
	return m, nil
}

func (r *ConnectionRepository) toEntity(m *models.ChannelConnection) (*entity.ChannelConnection, error) {
	secret, err := r.cipher.Decrypt(m.EncryptedSecret)
	if err != nil {
		return nil, apperror.Internal("decrypt secret").Wrap(err)
	}
	return &entity.ChannelConnection{
		ID:                 m.ID,
		TenantID:           m.TenantID,
		Type:               entity.Type(m.Type),
		Name:               m.Name,
		Status:             entity.Status(m.Status),
		BaseURL:            m.BaseURL,
		AuthType:           entity.AuthType(m.AuthType),
		Secret:             secret,
		WebhookVerifyToken: m.WebhookVerifyToken,
		DefaultSectorID:    m.DefaultSectorID,
		Enabled:            m.Enabled,
		AutomationEnabled:  m.AutomationEnabled,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}, nil
}

var _ repository.ConnectionRepository = (*ConnectionRepository)(nil)
