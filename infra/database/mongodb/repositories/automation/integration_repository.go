// Package automation is the Mongo implementation of the automation repositories.
package automation

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/secrets"
)

// IntegrationRepository implements repository.IntegrationRepository. The secret
// is encrypted at rest.
type IntegrationRepository struct {
	coll   *mongo.Collection
	cipher *secrets.Cipher
}

// NewIntegrationRepository builds the repository.
func NewIntegrationRepository(db *mongo.Database, cipher *secrets.Cipher) *IntegrationRepository {
	return &IntegrationRepository{coll: db.Collection("automation_integrations"), cipher: cipher}
}

func (r *IntegrationRepository) Create(ctx context.Context, i *entity.AutomationIntegration) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	enc, err := r.cipher.Encrypt(i.Secret)
	if err != nil {
		return apperror.Internal("encrypt secret").Wrap(err)
	}
	m := toModel(i, enc)
	_, err = r.coll.InsertOne(ctx, m)
	return mongodb.MapError(err)
}

func (r *IntegrationRepository) Update(ctx context.Context, i *entity.AutomationIntegration) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	enc, err := r.cipher.Encrypt(i.Secret)
	if err != nil {
		return apperror.Internal("encrypt secret").Wrap(err)
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": i.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":             i.Name,
			"base_url":         i.BaseURL,
			"auth_type":        i.AuthType,
			"encrypted_secret": enc,
			"enabled":          i.Enabled,
			"timeout_ms":       i.TimeoutMs,
			"updated_at":       i.UpdatedAt,
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

func (r *IntegrationRepository) Delete(ctx context.Context, id string) error {
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

func (r *IntegrationRepository) FindByID(ctx context.Context, id string) (*entity.AutomationIntegration, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.AutomationIntegration
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return r.toEntity(&m)
}

func (r *IntegrationRepository) FindEnabled(ctx context.Context) (*entity.AutomationIntegration, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.AutomationIntegration
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "enabled": true}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return r.toEntity(&m)
}

func (r *IntegrationRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.AutomationIntegration, error) {
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
	var out []*entity.AutomationIntegration
	for c.Next(ctx) {
		var m models.AutomationIntegration
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

func toModel(i *entity.AutomationIntegration, encryptedSecret string) models.AutomationIntegration {
	m := models.AutomationIntegration{
		Name:            i.Name,
		BaseURL:         i.BaseURL,
		AuthType:        i.AuthType,
		EncryptedSecret: encryptedSecret,
		Enabled:         i.Enabled,
		TimeoutMs:       i.TimeoutMs,
	}
	m.ID = i.ID
	m.TenantID = i.TenantID
	m.CreatedAt = i.CreatedAt
	m.UpdatedAt = i.UpdatedAt
	return m
}

func (r *IntegrationRepository) toEntity(m *models.AutomationIntegration) (*entity.AutomationIntegration, error) {
	secret, err := r.cipher.Decrypt(m.EncryptedSecret)
	if err != nil {
		return nil, apperror.Internal("decrypt secret").Wrap(err)
	}
	return &entity.AutomationIntegration{
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

var _ repository.IntegrationRepository = (*IntegrationRepository)(nil)
