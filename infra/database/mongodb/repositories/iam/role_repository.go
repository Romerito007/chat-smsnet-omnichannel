// Package iam is the Mongo implementation of the IAM repositories (users and
// roles). Every operation is scoped by the tenant taken from the context, never
// from arguments, enforcing isolation at the data layer.
package iam

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// RoleRepository implements repository.RoleRepository over MongoDB.
type RoleRepository struct {
	coll *mongo.Collection
}

// NewRoleRepository builds the repository.
func NewRoleRepository(db *mongo.Database) *RoleRepository {
	return &RoleRepository{coll: db.Collection("roles")}
}

func (r *RoleRepository) Create(ctx context.Context, role *entity.Role) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, roleToModel(role))
	return mongodb.MapError(err)
}

func (r *RoleRepository) Update(ctx context.Context, role *entity.Role) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": role.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":         role.Name,
			"permissions":  permsToStrings(role.Permissions),
			"sector_scope": string(role.SectorScope),
			"updated_at":   role.UpdatedAt,
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

func (r *RoleRepository) Delete(ctx context.Context, id string) error {
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

func (r *RoleRepository) FindByID(ctx context.Context, id string) (*entity.Role, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Role
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return roleToEntity(&m), nil
}

func (r *RoleRepository) FindByIDs(ctx context.Context, ids []string) ([]*entity.Role, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	cur, err := r.coll.Find(ctx, bson.M{"tenant_id": tenantID, "_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	return decodeRoles(ctx, cur)
}

func (r *RoleRepository) FindByName(ctx context.Context, name string) (*entity.Role, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Role
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "name": name}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return roleToEntity(&m), nil
}

func (r *RoleRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.Role, error) {
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
	return decodeRoles(ctx, c)
}

func decodeRoles(ctx context.Context, cur *mongo.Cursor) ([]*entity.Role, error) {
	defer func() { _ = cur.Close(ctx) }()
	var out []*entity.Role
	for cur.Next(ctx) {
		var m models.Role
		if err := cur.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, roleToEntity(&m))
	}
	return out, mongodb.MapError(cur.Err())
}

func roleToModel(r *entity.Role) models.Role {
	m := models.Role{
		Name:        r.Name,
		Permissions: permsToStrings(r.Permissions),
		SectorScope: string(r.SectorScope),
	}
	m.ID = r.ID
	m.TenantID = r.TenantID
	m.CreatedAt = r.CreatedAt
	m.UpdatedAt = r.UpdatedAt
	return m
}

func roleToEntity(m *models.Role) *entity.Role {
	return &entity.Role{
		ID:          m.ID,
		TenantID:    m.TenantID,
		Name:        m.Name,
		Permissions: stringsToPerms(m.Permissions),
		SectorScope: authz.SectorScope(m.SectorScope),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func permsToStrings(perms []authz.Permission) []string {
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = string(p)
	}
	return out
}

func stringsToPerms(ss []string) []authz.Permission {
	out := make([]authz.Permission, 0, len(ss))
	for _, s := range ss {
		out = append(out, authz.Permission(s))
	}
	return out
}

var _ repository.RoleRepository = (*RoleRepository)(nil)
