package sla

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sla/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/sla/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// TrackingRepository implements repository.TrackingRepository.
type TrackingRepository struct {
	coll *mongo.Collection
}

// NewTrackingRepository builds the repository.
func NewTrackingRepository(db *mongo.Database) *TrackingRepository {
	return &TrackingRepository{coll: db.Collection("sla_trackings")}
}

func (r *TrackingRepository) Create(ctx context.Context, t *entity.SLATracking) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toTrackingModel(t))
	return mongodb.MapError(err)
}

func (r *TrackingRepository) Update(ctx context.Context, t *entity.SLATracking) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": t.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"first_response_at":       t.FirstResponseAt,
			"resolved_at":             t.ResolvedAt,
			"first_response_breached": t.FirstResponseBreached,
			"resolution_breached":     t.ResolutionBreached,
			"first_response_warned":   t.FirstResponseWarned,
			"resolution_warned":       t.ResolutionWarned,
			"status":                  string(t.Status),
			"updated_at":              t.UpdatedAt,
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

func (r *TrackingRepository) FindByConversation(ctx context.Context, conversationID string) (*entity.SLATracking, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.SLATracking
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "conversation_id": conversationID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toTrackingEntity(&m), nil
}

func (r *TrackingRepository) ListAtRisk(ctx context.Context, page shared.PageRequest) ([]*entity.SLATracking, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	filter := mongodb.ApplyKeyset(bson.M{
		"tenant_id": tenantID,
		"status":    string(entity.StatusRunning),
		"$or": bson.A{
			bson.M{"first_response_warned": true},
			bson.M{"resolution_warned": true},
			bson.M{"first_response_breached": true},
			bson.M{"resolution_breached": true},
		},
	}, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	return r.query(ctx, filter, opts)
}

func (r *TrackingRepository) ListRunningAcrossTenants(ctx context.Context, limit int) ([]*entity.SLATracking, error) {
	// System-wide: intentionally NOT tenant-scoped (the sla.check job runs for all
	// tenants).
	if limit <= 0 {
		limit = 1000
	}
	opts := options.Find().SetLimit(int64(limit))
	return r.query(ctx, bson.M{"status": string(entity.StatusRunning)}, opts)
}

func (r *TrackingRepository) query(ctx context.Context, filter bson.M, opts *options.FindOptions) ([]*entity.SLATracking, error) {
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.SLATracking
	for c.Next(ctx) {
		var m models.SLATracking
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toTrackingEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toTrackingModel(t *entity.SLATracking) models.SLATracking {
	return models.SLATracking{
		ID:                     t.ID,
		TenantID:               t.TenantID,
		ConversationID:         t.ConversationID,
		PolicyID:               t.PolicyID,
		SectorID:               t.SectorID,
		FirstResponseDueAt:     t.FirstResponseDueAt,
		FirstResponseWarnAt:    t.FirstResponseWarnAt,
		ResolutionDueAt:        t.ResolutionDueAt,
		ResolutionWarnAt:       t.ResolutionWarnAt,
		FirstResponseAt:        t.FirstResponseAt,
		ResolvedAt:             t.ResolvedAt,
		FirstResponseBreached:  t.FirstResponseBreached,
		ResolutionBreached:     t.ResolutionBreached,
		FirstResponseWarned:    t.FirstResponseWarned,
		ResolutionWarned:       t.ResolutionWarned,
		PauseOnWaitingCustomer: t.PauseOnWaitingCustomer,
		Status:                 string(t.Status),
		CreatedAt:              t.CreatedAt,
		UpdatedAt:              t.UpdatedAt,
	}
}

func toTrackingEntity(m *models.SLATracking) *entity.SLATracking {
	return &entity.SLATracking{
		ID:                     m.ID,
		TenantID:               m.TenantID,
		ConversationID:         m.ConversationID,
		PolicyID:               m.PolicyID,
		SectorID:               m.SectorID,
		FirstResponseDueAt:     m.FirstResponseDueAt,
		FirstResponseWarnAt:    m.FirstResponseWarnAt,
		ResolutionDueAt:        m.ResolutionDueAt,
		ResolutionWarnAt:       m.ResolutionWarnAt,
		FirstResponseAt:        m.FirstResponseAt,
		ResolvedAt:             m.ResolvedAt,
		FirstResponseBreached:  m.FirstResponseBreached,
		ResolutionBreached:     m.ResolutionBreached,
		FirstResponseWarned:    m.FirstResponseWarned,
		ResolutionWarned:       m.ResolutionWarned,
		PauseOnWaitingCustomer: m.PauseOnWaitingCustomer,
		Status:                 entity.TrackingStatus(m.Status),
		CreatedAt:              m.CreatedAt,
		UpdatedAt:              m.UpdatedAt,
	}
}

var _ repository.TrackingRepository = (*TrackingRepository)(nil)
