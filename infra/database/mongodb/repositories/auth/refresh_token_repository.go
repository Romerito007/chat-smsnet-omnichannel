// Package auth is the Mongo implementation of the auth repositories.
package auth

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// RefreshTokenRepository implements repository.RefreshTokenRepository.
type RefreshTokenRepository struct {
	coll *mongo.Collection
}

// NewRefreshTokenRepository builds the repository.
func NewRefreshTokenRepository(db *mongo.Database) *RefreshTokenRepository {
	return &RefreshTokenRepository{coll: db.Collection("refresh_tokens")}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, t *entity.RefreshToken) error {
	_, err := r.coll.InsertOne(ctx, toModel(t))
	return mongodb.MapError(err)
}

// FindByHash looks a token up by its hash alone (pre-auth, not tenant-scoped).
func (r *RefreshTokenRepository) FindByHash(ctx context.Context, tokenHash string) (*entity.RefreshToken, error) {
	var m models.RefreshToken
	if err := r.coll.FindOne(ctx, bson.M{"token_hash": tokenHash}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": id, "revoked_at": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"revoked_at": now}},
	)
	return mongodb.MapError(err)
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, tenantID, userID string) error {
	now := time.Now().UTC()
	_, err := r.coll.UpdateMany(ctx,
		bson.M{"tenant_id": tenantID, "user_id": userID, "revoked_at": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"revoked_at": now}},
	)
	return mongodb.MapError(err)
}

func toModel(t *entity.RefreshToken) models.RefreshToken {
	return models.RefreshToken{
		ID:        t.ID,
		TenantID:  t.TenantID,
		UserID:    t.UserID,
		TokenHash: t.TokenHash,
		ExpiresAt: t.ExpiresAt,
		RevokedAt: t.RevokedAt,
		UserAgent: t.UserAgent,
		IP:        t.IP,
		CreatedAt: t.CreatedAt,
	}
}

func toEntity(m *models.RefreshToken) *entity.RefreshToken {
	return &entity.RefreshToken{
		ID:        m.ID,
		TenantID:  m.TenantID,
		UserID:    m.UserID,
		TokenHash: m.TokenHash,
		ExpiresAt: m.ExpiresAt,
		RevokedAt: m.RevokedAt,
		UserAgent: m.UserAgent,
		IP:        m.IP,
		CreatedAt: m.CreatedAt,
	}
}

var _ repository.RefreshTokenRepository = (*RefreshTokenRepository)(nil)
