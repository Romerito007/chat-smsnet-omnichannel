// Package copilot is the Mongo implementation of the copilot repositories
// (per-tenant config and the AI usage log).
package copilot

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/secrets"
)

// ConfigRepository implements repository.ConfigRepository. The per-tenant API key
// is encrypted on write and decrypted on read so plaintext is never persisted.
type ConfigRepository struct {
	coll   *mongo.Collection
	cipher *secrets.Cipher
}

// NewConfigRepository builds the repository.
func NewConfigRepository(db *mongo.Database, cipher *secrets.Cipher) *ConfigRepository {
	return &ConfigRepository{coll: db.Collection("copilot_configs"), cipher: cipher}
}

func (r *ConfigRepository) Create(ctx context.Context, c *entity.AIConfig) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	m, err := r.toModel(c)
	if err != nil {
		return apperror.Internal("encrypt api key").Wrap(err)
	}
	_, err = r.coll.InsertOne(ctx, m)
	return mongodb.MapError(err)
}

func (r *ConfigRepository) Update(ctx context.Context, c *entity.AIConfig) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	enc, err := r.cipher.Encrypt(c.APIKey)
	if err != nil {
		return apperror.Internal("encrypt api key").Wrap(err)
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": c.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"provider":          c.Provider,
			"model":             c.Model,
			"encrypted_api_key": enc,
			"base_url":          c.BaseURL,
			"enabled":           c.Enabled,
			"updated_at":        c.UpdatedAt,
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

func (r *ConfigRepository) FindByTenant(ctx context.Context) (*entity.AIConfig, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.AIConfig
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return r.toEntity(&m)
}

func (r *ConfigRepository) toModel(c *entity.AIConfig) (models.AIConfig, error) {
	enc, err := r.cipher.Encrypt(c.APIKey)
	if err != nil {
		return models.AIConfig{}, err
	}
	m := models.AIConfig{
		Provider:        string(c.Provider),
		Model:           c.Model,
		EncryptedAPIKey: enc,
		BaseURL:         c.BaseURL,
		Enabled:         c.Enabled,
	}
	m.ID = c.ID
	m.TenantID = c.TenantID
	m.CreatedAt = c.CreatedAt
	m.UpdatedAt = c.UpdatedAt
	return m, nil
}

func (r *ConfigRepository) toEntity(m *models.AIConfig) (*entity.AIConfig, error) {
	key, err := r.cipher.Decrypt(m.EncryptedAPIKey)
	if err != nil {
		return nil, apperror.Internal("decrypt api key").Wrap(err)
	}
	return &entity.AIConfig{
		ID:        m.ID,
		TenantID:  m.TenantID,
		Provider:  entity.Provider(m.Provider),
		Model:     m.Model,
		APIKey:    key,
		BaseURL:   m.BaseURL,
		Enabled:   m.Enabled,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

var _ repository.ConfigRepository = (*ConfigRepository)(nil)
