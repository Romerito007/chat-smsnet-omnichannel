// Package sla is the Mongo implementation of the SLA repositories (policies and
// per-conversation tracking).
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

// PolicyRepository implements repository.PolicyRepository.
type PolicyRepository struct {
	coll *mongo.Collection
}

// NewPolicyRepository builds the repository.
func NewPolicyRepository(db *mongo.Database) *PolicyRepository {
	return &PolicyRepository{coll: db.Collection("sla_policies")}
}

func (r *PolicyRepository) Create(ctx context.Context, p *entity.SLAPolicy) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toPolicyModel(p))
	return mongodb.MapError(err)
}

func (r *PolicyRepository) Update(ctx context.Context, p *entity.SLAPolicy) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": p.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":                          p.Name,
			"sector_ids":                    p.SectorIDs,
			"priority":                      p.Priority,
			"channel":                       p.Channel,
			"first_response_target_seconds": p.FirstResponseTargetSec,
			"resolution_target_seconds":     p.ResolutionTargetSec,
			"business_hours_only":           p.BusinessHoursOnly,
			"warning_threshold_percent":     p.WarningThresholdPct,
			"pause_on_waiting_customer":     p.PauseOnWaitingCustomer,
			"enabled":                       p.Enabled,
			"updated_at":                    p.UpdatedAt,
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

func (r *PolicyRepository) Delete(ctx context.Context, id string) error {
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

func (r *PolicyRepository) FindByID(ctx context.Context, id string) (*entity.SLAPolicy, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.SLAPolicy
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toPolicyEntity(&m), nil
}

func (r *PolicyRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.SLAPolicy, error) {
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
	return r.query(ctx, filter, opts)
}

func (r *PolicyRepository) ListEnabled(ctx context.Context) ([]*entity.SLAPolicy, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	return r.query(ctx, bson.M{"tenant_id": tenantID, "enabled": true}, nil)
}

func (r *PolicyRepository) query(ctx context.Context, filter bson.M, opts *options.FindOptions) ([]*entity.SLAPolicy, error) {
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer c.Close(ctx)
	var out []*entity.SLAPolicy
	for c.Next(ctx) {
		var m models.SLAPolicy
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toPolicyEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toPolicyModel(p *entity.SLAPolicy) models.SLAPolicy {
	m := models.SLAPolicy{
		Name:                   p.Name,
		SectorIDs:              p.SectorIDs,
		Priority:               p.Priority,
		Channel:                p.Channel,
		FirstResponseTargetSec: p.FirstResponseTargetSec,
		ResolutionTargetSec:    p.ResolutionTargetSec,
		BusinessHoursOnly:      p.BusinessHoursOnly,
		WarningThresholdPct:    p.WarningThresholdPct,
		PauseOnWaitingCustomer: p.PauseOnWaitingCustomer,
		Enabled:                p.Enabled,
	}
	m.ID = p.ID
	m.TenantID = p.TenantID
	m.CreatedAt = p.CreatedAt
	m.UpdatedAt = p.UpdatedAt
	return m
}

func toPolicyEntity(m *models.SLAPolicy) *entity.SLAPolicy {
	return &entity.SLAPolicy{
		ID:                     m.ID,
		TenantID:               m.TenantID,
		Name:                   m.Name,
		SectorIDs:              m.SectorIDs,
		Priority:               m.Priority,
		Channel:                m.Channel,
		FirstResponseTargetSec: m.FirstResponseTargetSec,
		ResolutionTargetSec:    m.ResolutionTargetSec,
		BusinessHoursOnly:      m.BusinessHoursOnly,
		WarningThresholdPct:    m.WarningThresholdPct,
		PauseOnWaitingCustomer: m.PauseOnWaitingCustomer,
		Enabled:                m.Enabled,
		CreatedAt:              m.CreatedAt,
		UpdatedAt:              m.UpdatedAt,
	}
}

var _ repository.PolicyRepository = (*PolicyRepository)(nil)
