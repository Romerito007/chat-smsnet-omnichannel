// Package copilot is the Mongo implementation of the copilot repositories
// (per-tenant config and the AI usage log).
package copilot

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// ConfigRepository implements repository.ConfigRepository.
type ConfigRepository struct {
	coll *mongo.Collection
}

// NewConfigRepository builds the repository.
func NewConfigRepository(db *mongo.Database) *ConfigRepository {
	return &ConfigRepository{coll: db.Collection("copilot_configs")}
}

func (r *ConfigRepository) Create(ctx context.Context, c *entity.AIConfig) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toModel(c))
	return mongodb.MapError(err)
}

func (r *ConfigRepository) Update(ctx context.Context, c *entity.AIConfig) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": c.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"provider":                c.Provider,
			"model":                   c.Model,
			"temperature":             c.Temperature,
			"max_tokens":              c.MaxTokens,
			"allow_customer_data":     c.AllowCustomerData,
			"allow_financial_data":    c.AllowFinancialData,
			"allow_monitoring_data":   c.AllowMonitoringData,
			"human_approval_required": c.HumanApprovalRequired,
			"enabled":                 c.Enabled,
			"updated_at":              c.UpdatedAt,
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

func (r *ConfigRepository) FindByTenant(ctx context.Context) (*entity.AIConfig, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.AIConfig
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func toModel(c *entity.AIConfig) models.AIConfig {
	m := models.AIConfig{
		Provider:              string(c.Provider),
		Model:                 c.Model,
		Temperature:           c.Temperature,
		MaxTokens:             c.MaxTokens,
		AllowCustomerData:     c.AllowCustomerData,
		AllowFinancialData:    c.AllowFinancialData,
		AllowMonitoringData:   c.AllowMonitoringData,
		HumanApprovalRequired: c.HumanApprovalRequired,
		Enabled:               c.Enabled,
	}
	m.ID = c.ID
	m.TenantID = c.TenantID
	m.CreatedAt = c.CreatedAt
	m.UpdatedAt = c.UpdatedAt
	return m
}

func toEntity(m *models.AIConfig) *entity.AIConfig {
	return &entity.AIConfig{
		ID:                    m.ID,
		TenantID:              m.TenantID,
		Provider:              entity.Provider(m.Provider),
		Model:                 m.Model,
		Temperature:           m.Temperature,
		MaxTokens:             m.MaxTokens,
		AllowCustomerData:     m.AllowCustomerData,
		AllowFinancialData:    m.AllowFinancialData,
		AllowMonitoringData:   m.AllowMonitoringData,
		HumanApprovalRequired: m.HumanApprovalRequired,
		Enabled:               m.Enabled,
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
	}
}

var _ repository.ConfigRepository = (*ConfigRepository)(nil)
