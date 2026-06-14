// Package automationrules is the Mongo implementation of the automation-rules
// repositories.
package automationrules

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automationrules/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// RuleRepository implements repository.RuleRepository over the automation_rules
// collection.
type RuleRepository struct {
	coll *mongo.Collection
}

// NewRuleRepository builds the repository.
func NewRuleRepository(db *mongo.Database) *RuleRepository {
	return &RuleRepository{coll: db.Collection("automation_rules")}
}

func (r *RuleRepository) Create(ctx context.Context, rule *entity.AutomationRule) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toModel(rule))
	return mongodb.MapError(err)
}

func (r *RuleRepository) Update(ctx context.Context, rule *entity.AutomationRule) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	m := toModel(rule)
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": rule.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":        m.Name,
			"description": m.Description,
			"event":       m.Event,
			"enabled":     m.Enabled,
			"conditions":  m.Conditions,
			"actions":     m.Actions,
			"updated_at":  rule.UpdatedAt,
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

func (r *RuleRepository) Delete(ctx context.Context, id string) error {
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

func (r *RuleRepository) FindByID(ctx context.Context, id string) (*entity.AutomationRule, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.AutomationRule
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *RuleRepository) List(ctx context.Context) ([]*entity.AutomationRule, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	return r.find(ctx, bson.M{"tenant_id": tenantID})
}

func (r *RuleRepository) ListEnabledByEvent(ctx context.Context, event entity.RuleEvent) ([]*entity.AutomationRule, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	return r.find(ctx, bson.M{"tenant_id": tenantID, "enabled": true, "event": string(event)})
}

func (r *RuleRepository) FindOneByWebhook(ctx context.Context, webhookID string) (*entity.AutomationRule, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.AutomationRule
	err = r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "actions.webhook_id": webhookID}).Decode(&m)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *RuleRepository) find(ctx context.Context, filter bson.M) ([]*entity.AutomationRule, error) {
	cur, err := r.coll.Find(ctx, filter)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []*entity.AutomationRule
	for cur.Next(ctx) {
		var m models.AutomationRule
		if err := cur.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toEntity(&m))
	}
	return out, mongodb.MapError(cur.Err())
}

func toModel(r *entity.AutomationRule) models.AutomationRule {
	conds := make([]models.RuleCondition, len(r.Conditions))
	for i, c := range r.Conditions {
		conds[i] = models.RuleCondition{Field: string(c.Field), Operator: string(c.Operator), Value: c.Value}
	}
	acts := make([]models.RuleAction, len(r.Actions))
	for i, a := range r.Actions {
		acts[i] = models.RuleAction{Type: string(a.Type), WebhookID: a.WebhookID}
	}
	m := models.AutomationRule{
		Name:        r.Name,
		Description: r.Description,
		Event:       string(r.Event),
		Enabled:     r.Enabled,
		Conditions:  conds,
		Actions:     acts,
	}
	m.ID = r.ID
	m.TenantID = r.TenantID
	m.CreatedAt = r.CreatedAt
	m.UpdatedAt = r.UpdatedAt
	return m
}

func toEntity(m *models.AutomationRule) *entity.AutomationRule {
	conds := make([]entity.Condition, len(m.Conditions))
	for i, c := range m.Conditions {
		conds[i] = entity.Condition{Field: entity.ConditionField(c.Field), Operator: entity.ConditionOperator(c.Operator), Value: c.Value}
	}
	acts := make([]entity.Action, len(m.Actions))
	for i, a := range m.Actions {
		acts[i] = entity.Action{Type: entity.ActionType(a.Type), WebhookID: a.WebhookID}
	}
	return &entity.AutomationRule{
		ID:          m.ID,
		TenantID:    m.TenantID,
		Name:        m.Name,
		Description: m.Description,
		Event:       entity.RuleEvent(m.Event),
		Enabled:     m.Enabled,
		Conditions:  conds,
		Actions:     acts,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

var _ repository.RuleRepository = (*RuleRepository)(nil)
