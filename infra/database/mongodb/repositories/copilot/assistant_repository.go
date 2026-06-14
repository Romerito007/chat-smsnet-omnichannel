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

// AssistantRepository implements repository.AssistantRepository over the
// copilot_assistants collection.
type AssistantRepository struct {
	coll *mongo.Collection
}

// NewAssistantRepository builds the repository.
func NewAssistantRepository(db *mongo.Database) *AssistantRepository {
	return &AssistantRepository{coll: db.Collection("copilot_assistants")}
}

func (r *AssistantRepository) Create(ctx context.Context, a *entity.Assistant) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toAssistantModel(a))
	return mongodb.MapError(err)
}

func (r *AssistantRepository) Update(ctx context.Context, a *entity.Assistant) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	m := toAssistantModel(a)
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": a.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":           m.Name,
			"channel_ids":    m.ChannelIDs,
			"isp_profile_id": m.ISPProfileID,
			"mcp_server_id":  m.MCPServerID,
			"enabled":        m.Enabled,
			"updated_at":     a.UpdatedAt,
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

func (r *AssistantRepository) Delete(ctx context.Context, id string) error {
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

func (r *AssistantRepository) FindByID(ctx context.Context, id string) (*entity.Assistant, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Assistant
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toAssistantEntity(&m), nil
}

func (r *AssistantRepository) List(ctx context.Context) ([]*entity.Assistant, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := r.coll.Find(ctx, bson.M{"tenant_id": tenantID})
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []*entity.Assistant
	for cur.Next(ctx) {
		var m models.Assistant
		if err := cur.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toAssistantEntity(&m))
	}
	return out, mongodb.MapError(cur.Err())
}

func (r *AssistantRepository) FindByChannelID(ctx context.Context, channelID string) (*entity.Assistant, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Assistant
	err = r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "enabled": true, "channel_ids": channelID}).Decode(&m)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	return toAssistantEntity(&m), nil
}

func (r *AssistantRepository) CountByISPProfile(ctx context.Context, ispProfileID string) (int, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, err
	}
	n, err := r.coll.CountDocuments(ctx, bson.M{"tenant_id": tenantID, "isp_profile_id": ispProfileID})
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	return int(n), nil
}

func toAssistantModel(a *entity.Assistant) models.Assistant {
	m := models.Assistant{
		Name:         a.Name,
		ChannelIDs:   a.ChannelIDs,
		ISPProfileID: a.ISPProfileID,
		MCPServerID:  a.MCPServerID,
		Enabled:      a.Enabled,
	}
	m.ID = a.ID
	m.TenantID = a.TenantID
	m.CreatedAt = a.CreatedAt
	m.UpdatedAt = a.UpdatedAt
	return m
}

func toAssistantEntity(m *models.Assistant) *entity.Assistant {
	return &entity.Assistant{
		ID:           m.ID,
		TenantID:     m.TenantID,
		Name:         m.Name,
		ChannelIDs:   m.ChannelIDs,
		ISPProfileID: m.ISPProfileID,
		MCPServerID:  m.MCPServerID,
		Enabled:      m.Enabled,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

var _ repository.AssistantRepository = (*AssistantRepository)(nil)
