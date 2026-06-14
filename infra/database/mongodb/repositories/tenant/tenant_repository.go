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

// Create inserts a new tenant (self-service signup).
func (r *Repository) Create(ctx context.Context, t *entity.Tenant) error {
	_, err := r.coll.InsertOne(ctx, models.Tenant{
		ID:          t.ID,
		Name:        t.Name,
		Status:      string(t.Status),
		ExternalRef: t.ExternalRef,
		Settings:    t.Settings,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	})
	return mongodb.MapError(err)
}

// FindByID returns the tenant or a not_found error.
func (r *Repository) FindByID(ctx context.Context, id string) (*entity.Tenant, error) {
	var m models.Tenant
	if err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

// FindByExternalRef returns the tenant with the given provisioner external_ref,
// or a not_found error. Used for durable provisioning idempotency.
func (r *Repository) FindByExternalRef(ctx context.Context, ref string) (*entity.Tenant, error) {
	var m models.Tenant
	if err := r.coll.FindOne(ctx, bson.M{"external_ref": ref}).Decode(&m); err != nil {
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

// ListActive returns every active tenant.
func (r *Repository) ListActive(ctx context.Context) ([]*entity.Tenant, error) {
	c, err := r.coll.Find(ctx, bson.M{"status": string(entity.StatusActive)})
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.Tenant
	for c.Next(ctx) {
		var m models.Tenant
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toEntity(m *models.Tenant) *entity.Tenant {
	return &entity.Tenant{
		ID:          m.ID,
		Name:        m.Name,
		Status:      entity.Status(m.Status),
		ExternalRef: m.ExternalRef,
		Settings:    m.Settings,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

var _ repository.TenantRepository = (*Repository)(nil)
