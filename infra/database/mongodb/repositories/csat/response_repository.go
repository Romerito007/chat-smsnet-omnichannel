package csat

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/csat/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/csat/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// ResponseRepository implements repository.ResponseRepository.
type ResponseRepository struct {
	coll *mongo.Collection
}

// NewResponseRepository builds the repository.
func NewResponseRepository(db *mongo.Database) *ResponseRepository {
	return &ResponseRepository{coll: db.Collection("csat_responses")}
}

func (r *ResponseRepository) Create(ctx context.Context, resp *entity.CSATResponse) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toResponseModel(resp))
	return mongodb.MapError(err)
}

func (r *ResponseRepository) Update(ctx context.Context, resp *entity.CSATResponse) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": resp.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"score":        resp.Score,
			"comment":      resp.Comment,
			"sent_at":      resp.SentAt,
			"responded_at": resp.RespondedAt,
			"status":       string(resp.Status),
			"updated_at":   resp.UpdatedAt,
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

func (r *ResponseRepository) FindByID(ctx context.Context, id string) (*entity.CSATResponse, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.CSATResponse
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toResponseEntity(&m), nil
}

// FindByToken is intentionally NOT tenant-scoped: the public answer endpoint has
// no tenant context. The token is an unguessable, globally-unique handle.
func (r *ResponseRepository) FindByToken(ctx context.Context, token string) (*entity.CSATResponse, error) {
	var m models.CSATResponse
	if err := r.coll.FindOne(ctx, bson.M{"token": token}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toResponseEntity(&m), nil
}

func (r *ResponseRepository) FindByConversation(ctx context.Context, conversationID string) (*entity.CSATResponse, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.CSATResponse
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "conversation_id": conversationID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toResponseEntity(&m), nil
}

func (r *ResponseRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.CSATResponse, error) {
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
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.CSATResponse
	for c.Next(ctx) {
		var m models.CSATResponse
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toResponseEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toResponseModel(r *entity.CSATResponse) models.CSATResponse {
	return models.CSATResponse{
		ID: r.ID, TenantID: r.TenantID, ConversationID: r.ConversationID,
		ContactID: r.ContactID, SurveyID: r.SurveyID, AgentID: r.AgentID, Token: r.Token,
		Score: r.Score, Comment: r.Comment, SentAt: r.SentAt, RespondedAt: r.RespondedAt,
		Status: string(r.Status), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func toResponseEntity(m *models.CSATResponse) *entity.CSATResponse {
	return &entity.CSATResponse{
		ID: m.ID, TenantID: m.TenantID, ConversationID: m.ConversationID,
		ContactID: m.ContactID, SurveyID: m.SurveyID, AgentID: m.AgentID, Token: m.Token,
		Score: m.Score, Comment: m.Comment, SentAt: m.SentAt, RespondedAt: m.RespondedAt,
		Status: entity.Status(m.Status), CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

var _ repository.ResponseRepository = (*ResponseRepository)(nil)
