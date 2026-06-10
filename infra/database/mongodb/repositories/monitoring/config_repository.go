// Package monitoring is the Mongo implementation of the monitoring repositories
// (config and the minimal query log).
package monitoring

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/monitoring/repository"
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
	return &ConfigRepository{coll: db.Collection("monitoring_configs"), cipher: cipher}
}

func (r *ConfigRepository) Create(ctx context.Context, c *entity.MonitoringIntegrationConfig) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	enc, err := r.cipher.Encrypt(c.Secret)
	if err != nil {
		return apperror.Internal("encrypt secret").Wrap(err)
	}
	m := models.MonitoringIntegrationConfig{
		Name:            c.Name,
		BaseURL:         c.BaseURL,
		AuthType:        c.AuthType,
		EncryptedSecret: enc,
		Enabled:         c.Enabled,
		TimeoutMs:       c.TimeoutMs,
	}
	m.ID = c.ID
	m.TenantID = c.TenantID
	m.CreatedAt = c.CreatedAt
	m.UpdatedAt = c.UpdatedAt
	_, err = r.coll.InsertOne(ctx, m)
	return mongodb.MapError(err)
}

func (r *ConfigRepository) Update(ctx context.Context, c *entity.MonitoringIntegrationConfig) error {
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

func (r *ConfigRepository) FindEnabled(ctx context.Context) (*entity.MonitoringIntegrationConfig, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.MonitoringIntegrationConfig
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "enabled": true}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	secret, err := r.cipher.Decrypt(m.EncryptedSecret)
	if err != nil {
		return nil, apperror.Internal("decrypt secret").Wrap(err)
	}
	return &entity.MonitoringIntegrationConfig{
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
