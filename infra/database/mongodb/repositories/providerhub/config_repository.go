// Package providerhub is the Mongo implementation of the providerhub repositories
// (config and the minimal query log).
package providerhub

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/secrets"
)

// ConfigRepository implements repository.ConfigRepository. The secret is
// encrypted at rest.
type ConfigRepository struct {
	coll   *mongo.Collection
	cipher *secrets.Cipher
}

// NewConfigRepository builds the repository.
func NewConfigRepository(db *mongo.Database, cipher *secrets.Cipher) *ConfigRepository {
	return &ConfigRepository{coll: db.Collection("providerhub_configs"), cipher: cipher}
}

func (r *ConfigRepository) Create(ctx context.Context, c *entity.ProviderIntegrationConfig) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	enc, err := r.cipher.Encrypt(c.Secret)
	if err != nil {
		return apperror.Internal("encrypt secret").Wrap(err)
	}
	_, err = r.coll.InsertOne(ctx, toModel(c, enc))
	return mongodb.MapError(err)
}

func (r *ConfigRepository) Update(ctx context.Context, c *entity.ProviderIntegrationConfig) error {
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
			"name":             c.Name,
			"base_url":         c.BaseURL,
			"auth_type":        c.AuthType,
			"encrypted_secret": enc,
			"enabled":          c.Enabled,
			"timeout_ms":       c.TimeoutMs,
			"updated_at":       c.UpdatedAt,
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

func (r *ConfigRepository) FindByID(ctx context.Context, id string) (*entity.ProviderIntegrationConfig, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.ProviderIntegrationConfig
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return r.toEntity(&m)
}

func (r *ConfigRepository) FindEnabled(ctx context.Context) (*entity.ProviderIntegrationConfig, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.ProviderIntegrationConfig
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "enabled": true}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return r.toEntity(&m)
}

func (r *ConfigRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.ProviderIntegrationConfig, error) {
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
	var out []*entity.ProviderIntegrationConfig
	for c.Next(ctx) {
		var m models.ProviderIntegrationConfig
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

func toModel(c *entity.ProviderIntegrationConfig, encryptedSecret string) models.ProviderIntegrationConfig {
	m := models.ProviderIntegrationConfig{
		Name:            c.Name,
		BaseURL:         c.BaseURL,
		AuthType:        c.AuthType,
		EncryptedSecret: encryptedSecret,
		Enabled:         c.Enabled,
		TimeoutMs:       c.TimeoutMs,
	}
	m.ID = c.ID
	m.TenantID = c.TenantID
	m.CreatedAt = c.CreatedAt
	m.UpdatedAt = c.UpdatedAt
	return m
}

func (r *ConfigRepository) toEntity(m *models.ProviderIntegrationConfig) (*entity.ProviderIntegrationConfig, error) {
	secret, err := r.cipher.Decrypt(m.EncryptedSecret)
	if err != nil {
		return nil, apperror.Internal("decrypt secret").Wrap(err)
	}
	return &entity.ProviderIntegrationConfig{
		ID:        m.ID,
		TenantID:  m.TenantID,
		Name:      m.Name,
		BaseURL:   m.BaseURL,
		AuthType:  m.AuthType,
		Secret:    secret,
		Enabled:   m.Enabled,
		TimeoutMs: m.TimeoutMs,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

var _ repository.ConfigRepository = (*ConfigRepository)(nil)
