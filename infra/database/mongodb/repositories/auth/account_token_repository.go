package auth

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/auth/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// ── email verification tokens ─────────────────────────────────────────────────

// EmailVerificationTokenRepository implements the matching domain port.
type EmailVerificationTokenRepository struct{ coll *mongo.Collection }

// NewEmailVerificationTokenRepository builds the repository.
func NewEmailVerificationTokenRepository(db *mongo.Database) *EmailVerificationTokenRepository {
	return &EmailVerificationTokenRepository{coll: db.Collection("email_verification_tokens")}
}

func (r *EmailVerificationTokenRepository) Create(ctx context.Context, t *entity.EmailVerificationToken) error {
	_, err := r.coll.InsertOne(ctx, models.EmailVerificationToken{
		ID: t.ID, TenantID: t.TenantID, UserID: t.UserID, TokenHash: t.TokenHash,
		ExpiresAt: t.ExpiresAt, UsedAt: t.UsedAt, CreatedAt: t.CreatedAt,
	})
	return mongodb.MapError(err)
}

// FindByHash resolves a token by hash (pre-auth, not tenant-scoped).
func (r *EmailVerificationTokenRepository) FindByHash(ctx context.Context, tokenHash string) (*entity.EmailVerificationToken, error) {
	var m models.EmailVerificationToken
	if err := r.coll.FindOne(ctx, bson.M{"token_hash": tokenHash}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return &entity.EmailVerificationToken{
		ID: m.ID, TenantID: m.TenantID, UserID: m.UserID, TokenHash: m.TokenHash,
		ExpiresAt: m.ExpiresAt, UsedAt: m.UsedAt, CreatedAt: m.CreatedAt,
	}, nil
}

func (r *EmailVerificationTokenRepository) MarkUsed(ctx context.Context, id string, usedAt time.Time) error {
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": id, "used_at": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"used_at": usedAt}},
	)
	return mongodb.MapError(err)
}

func (r *EmailVerificationTokenRepository) InvalidateForUser(ctx context.Context, userID string, usedAt time.Time) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	_, err = r.coll.UpdateMany(ctx,
		bson.M{"tenant_id": tenantID, "user_id": userID, "used_at": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"used_at": usedAt}},
	)
	return mongodb.MapError(err)
}

var _ repository.EmailVerificationTokenRepository = (*EmailVerificationTokenRepository)(nil)

// ── password reset tokens ─────────────────────────────────────────────────────

// PasswordResetTokenRepository implements the matching domain port.
type PasswordResetTokenRepository struct{ coll *mongo.Collection }

// NewPasswordResetTokenRepository builds the repository.
func NewPasswordResetTokenRepository(db *mongo.Database) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{coll: db.Collection("password_reset_tokens")}
}

func (r *PasswordResetTokenRepository) Create(ctx context.Context, t *entity.PasswordResetToken) error {
	_, err := r.coll.InsertOne(ctx, models.PasswordResetToken{
		ID: t.ID, TenantID: t.TenantID, UserID: t.UserID, TokenHash: t.TokenHash,
		ExpiresAt: t.ExpiresAt, UsedAt: t.UsedAt, CreatedAt: t.CreatedAt,
	})
	return mongodb.MapError(err)
}

func (r *PasswordResetTokenRepository) FindByHash(ctx context.Context, tokenHash string) (*entity.PasswordResetToken, error) {
	var m models.PasswordResetToken
	if err := r.coll.FindOne(ctx, bson.M{"token_hash": tokenHash}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return &entity.PasswordResetToken{
		ID: m.ID, TenantID: m.TenantID, UserID: m.UserID, TokenHash: m.TokenHash,
		ExpiresAt: m.ExpiresAt, UsedAt: m.UsedAt, CreatedAt: m.CreatedAt,
	}, nil
}

func (r *PasswordResetTokenRepository) MarkUsed(ctx context.Context, id string, usedAt time.Time) error {
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": id, "used_at": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"used_at": usedAt}},
	)
	return mongodb.MapError(err)
}

func (r *PasswordResetTokenRepository) InvalidateForUser(ctx context.Context, userID string, usedAt time.Time) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	_, err = r.coll.UpdateMany(ctx,
		bson.M{"tenant_id": tenantID, "user_id": userID, "used_at": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"used_at": usedAt}},
	)
	return mongodb.MapError(err)
}

var _ repository.PasswordResetTokenRepository = (*PasswordResetTokenRepository)(nil)

// ── invitations ───────────────────────────────────────────────────────────────

// InvitationRepository implements the matching domain port.
type InvitationRepository struct{ coll *mongo.Collection }

// NewInvitationRepository builds the repository.
func NewInvitationRepository(db *mongo.Database) *InvitationRepository {
	return &InvitationRepository{coll: db.Collection("invitations")}
}

func (r *InvitationRepository) Create(ctx context.Context, i *entity.Invitation) error {
	_, err := r.coll.InsertOne(ctx, models.Invitation{
		ID: i.ID, TenantID: i.TenantID, Email: i.Email, RoleIDs: i.RoleIDs, SectorIDs: i.SectorIDs,
		TokenHash: i.TokenHash, ExpiresAt: i.ExpiresAt, AcceptedAt: i.AcceptedAt,
		InvitedBy: i.InvitedBy, CreatedAt: i.CreatedAt,
	})
	return mongodb.MapError(err)
}

func (r *InvitationRepository) FindByHash(ctx context.Context, tokenHash string) (*entity.Invitation, error) {
	var m models.Invitation
	if err := r.coll.FindOne(ctx, bson.M{"token_hash": tokenHash}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return invitationToEntity(&m), nil
}

func (r *InvitationRepository) FindPendingByEmail(ctx context.Context, email string) (*entity.Invitation, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Invitation
	filter := bson.M{"tenant_id": tenantID, "email": email, "accepted_at": bson.M{"$exists": false}}
	if err := r.coll.FindOne(ctx, filter).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return invitationToEntity(&m), nil
}

func (r *InvitationRepository) MarkAccepted(ctx context.Context, id string, acceptedAt time.Time) error {
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": id, "accepted_at": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"accepted_at": acceptedAt}},
	)
	return mongodb.MapError(err)
}

func invitationToEntity(m *models.Invitation) *entity.Invitation {
	return &entity.Invitation{
		ID: m.ID, TenantID: m.TenantID, Email: m.Email, RoleIDs: m.RoleIDs, SectorIDs: m.SectorIDs,
		TokenHash: m.TokenHash, ExpiresAt: m.ExpiresAt, AcceptedAt: m.AcceptedAt,
		InvitedBy: m.InvitedBy, CreatedAt: m.CreatedAt,
	}
}

var _ repository.InvitationRepository = (*InvitationRepository)(nil)
