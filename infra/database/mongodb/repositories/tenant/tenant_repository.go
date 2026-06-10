// Package tenant is the Mongo implementation of the tenant repository.
package tenant

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/tenant/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// Repository implements repository.TenantRepository over MongoDB.
type Repository struct {
	coll *mongo.Collection
}

// New builds the repository.
func New(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("tenants")}
}

// FindByID returns the tenant or a not_found error.
func (r *Repository) FindByID(ctx context.Context, id string) (*entity.Tenant, error) {
	var m models.Tenant
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

// Update persists the mutable fields of the tenant.
func (r *Repository) Update(ctx context.Context, t *entity.Tenant) error {
	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": t.ID}, bson.M{"$set": bson.M{
		"name":       t.Name,
		"status":     string(t.Status),
		"settings":   t.Settings,
		"updated_at": t.UpdatedAt,
	}})
	return mongodb.MapError(err)
}

func toEntity(m *models.Tenant) *entity.Tenant {
	return &entity.Tenant{
		ID:        m.ID,
		Name:      m.Name,
		Status:    entity.Status(m.Status),
		Settings:  m.Settings,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

var _ repository.TenantRepository = (*Repository)(nil)
