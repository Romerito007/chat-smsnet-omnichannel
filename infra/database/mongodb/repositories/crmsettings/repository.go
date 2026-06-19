// Package crmsettings is the Mongo implementation of the CRM-settings repository.
// One document per tenant; every operation is scoped by the tenant from the context.
package crmsettings

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/crmsettings/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// Repository implements repository.CRMSettingsRepository over MongoDB.
type Repository struct {
	coll *mongo.Collection
}

// New builds the repository.
func New(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("crm_settings")}
}

func (r *Repository) Get(ctx context.Context) (*entity.CRMSettings, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.CRMSettings
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *Repository) Upsert(ctx context.Context, s *entity.CRMSettings) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	_, err = r.coll.UpdateOne(ctx,
		bson.M{"tenant_id": tenantID},
		bson.M{
			"$set": bson.M{
				"tasks_enabled":    s.TasksEnabled,
				"products_enabled": s.ProductsEnabled,
				"timeline_enabled": s.TimelineEnabled,
				"updated_at":       s.UpdatedAt,
			},
			"$setOnInsert": bson.M{"_id": tenantID, "tenant_id": tenantID, "created_at": s.UpdatedAt},
		},
		options.Update().SetUpsert(true),
	)
	return mongodb.MapError(err)
}

func toEntity(m *models.CRMSettings) *entity.CRMSettings {
	return &entity.CRMSettings{
		TenantID:        m.TenantID,
		TasksEnabled:    m.TasksEnabled,
		ProductsEnabled: m.ProductsEnabled,
		TimelineEnabled: m.TimelineEnabled,
		UpdatedAt:       m.UpdatedAt,
	}
}

var _ repository.CRMSettingsRepository = (*Repository)(nil)
