// Package channels is the Mongo implementation of the channels repositories
// (integrations and the inbound idempotency ledger).
package channels

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// IntegrationRepository implements repository.IntegrationRepository.
type IntegrationRepository struct {
	coll *mongo.Collection
}

// NewIntegrationRepository builds the repository.
func NewIntegrationRepository(db *mongo.Database) *IntegrationRepository {
	return &IntegrationRepository{coll: db.Collection("channel_integrations")}
}

func (r *IntegrationRepository) Create(ctx context.Context, i *entity.Integration) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, integrationToModel(i))
	return mongodb.MapError(err)
}

func (r *IntegrationRepository) FindByID(ctx context.Context, id string) (*entity.Integration, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Integration
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return integrationToEntity(&m), nil
}

// FindByIntegrationKey is intentionally not tenant-scoped (pre-auth inbound).
func (r *IntegrationRepository) FindByIntegrationKey(ctx context.Context, integrationKey string) (*entity.Integration, error) {
	var m models.Integration
	if err := r.coll.FindOne(ctx, bson.M{"integration_key": integrationKey}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return integrationToEntity(&m), nil
}

func (r *IntegrationRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.Integration, error) {
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
	var out []*entity.Integration
	for c.Next(ctx) {
		var m models.Integration
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, integrationToEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func integrationToModel(i *entity.Integration) models.Integration {
	m := models.Integration{
		Channel:           i.Channel,
		Name:              i.Name,
		IntegrationKey:    i.IntegrationKey,
		Secret:            i.Secret,
		Enabled:           i.Enabled,
		AutomationEnabled: i.AutomationEnabled,
		DefaultQueueID:    i.DefaultQueueID,
	}
	m.ID = i.ID
	m.TenantID = i.TenantID
	m.CreatedAt = i.CreatedAt
	m.UpdatedAt = i.UpdatedAt
	return m
}

func integrationToEntity(m *models.Integration) *entity.Integration {
	return &entity.Integration{
		ID:                m.ID,
		TenantID:          m.TenantID,
		Channel:           m.Channel,
		Name:              m.Name,
		IntegrationKey:    m.IntegrationKey,
		Secret:            m.Secret,
		Enabled:           m.Enabled,
		AutomationEnabled: m.AutomationEnabled,
		DefaultQueueID:    m.DefaultQueueID,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
}

var _ repository.IntegrationRepository = (*IntegrationRepository)(nil)
