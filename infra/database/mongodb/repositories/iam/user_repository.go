package iam

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/iam/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// UserRepository implements repository.UserRepository over MongoDB.
type UserRepository struct {
	coll *mongo.Collection
}

// NewUserRepository builds the repository.
func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{coll: db.Collection("users")}
}

func (r *UserRepository) Create(ctx context.Context, u *entity.User) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, userToModel(u))
	return mongodb.MapError(err)
}

func (r *UserRepository) Update(ctx context.Context, u *entity.User) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": u.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":                 u.Name,
			"password_hash":        u.PasswordHash,
			"status":               string(u.Status),
			"role_ids":             u.RoleIDs,
			"sector_ids":           u.SectorIDs,
			"max_concurrent_chats": u.MaxConcurrentChats,
			"updated_at":           u.UpdatedAt,
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

func (r *UserRepository) Delete(ctx context.Context, id string) error {
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

func (r *UserRepository) FindByID(ctx context.Context, id string) (*entity.User, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.User
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return userToEntity(&m), nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.User
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "email": email}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return userToEntity(&m), nil
}

// FindByEmailAnyTenant is deliberately not tenant-scoped (pre-auth login only).
func (r *UserRepository) FindByEmailAnyTenant(ctx context.Context, email string) (*entity.User, error) {
	var m models.User
	if err := r.coll.FindOne(ctx, bson.M{"email": email}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return userToEntity(&m), nil
}

func (r *UserRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.User, error) {
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
	var out []*entity.User
	for c.Next(ctx) {
		var m models.User
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, userToEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func userToModel(u *entity.User) models.User {
	m := models.User{
		Name:               u.Name,
		Email:              u.Email,
		PasswordHash:       u.PasswordHash,
		Status:             string(u.Status),
		RoleIDs:            u.RoleIDs,
		SectorIDs:          u.SectorIDs,
		MaxConcurrentChats: u.MaxConcurrentChats,
	}
	m.ID = u.ID
	m.TenantID = u.TenantID
	m.CreatedAt = u.CreatedAt
	m.UpdatedAt = u.UpdatedAt
	return m
}

func userToEntity(m *models.User) *entity.User {
	return &entity.User{
		ID:                 m.ID,
		TenantID:           m.TenantID,
		Name:               m.Name,
		Email:              m.Email,
		PasswordHash:       m.PasswordHash,
		Status:             entity.Status(m.Status),
		RoleIDs:            m.RoleIDs,
		SectorIDs:          m.SectorIDs,
		MaxConcurrentChats: m.MaxConcurrentChats,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
}

var _ repository.UserRepository = (*UserRepository)(nil)
