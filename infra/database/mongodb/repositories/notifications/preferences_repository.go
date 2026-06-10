package notifications

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/notifications/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// PreferencesRepository implements repository.PreferencesRepository.
type PreferencesRepository struct {
	coll *mongo.Collection
}

// NewPreferencesRepository builds the repository.
func NewPreferencesRepository(db *mongo.Database) *PreferencesRepository {
	return &PreferencesRepository{coll: db.Collection("notification_preferences")}
}

func (r *PreferencesRepository) FindByUser(ctx context.Context, userID string) (*entity.Preferences, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.NotificationPreferences
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "user_id": userID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toPrefsEntity(&m), nil
}

func (r *PreferencesRepository) Upsert(ctx context.Context, p *entity.Preferences) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	emailByType := make(map[string]bool, len(p.EmailByType))
	for t, v := range p.EmailByType {
		emailByType[string(t)] = v
	}
	id := tenantID + ":" + p.UserID
	_, err = r.coll.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"tenant_id":     tenantID,
			"user_id":       p.UserID,
			"email_by_type": emailByType,
			"updated_at":    p.UpdatedAt,
		}},
		options.Update().SetUpsert(true),
	)
	return mongodb.MapError(err)
}

func toPrefsEntity(m *models.NotificationPreferences) *entity.Preferences {
	byType := make(map[entity.Type]bool, len(m.EmailByType))
	for t, v := range m.EmailByType {
		byType[entity.Type(t)] = v
	}
	return &entity.Preferences{
		TenantID:    m.TenantID,
		UserID:      m.UserID,
		EmailByType: byType,
		UpdatedAt:   m.UpdatedAt,
	}
}

var _ repository.PreferencesRepository = (*PreferencesRepository)(nil)
