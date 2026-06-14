// Package customattributes is the Mongo implementation of the custom-attribute
// definition repository.
package customattributes

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/customattributes/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// DefinitionRepository implements repository.DefinitionRepository.
type DefinitionRepository struct {
	coll *mongo.Collection
}

// NewDefinitionRepository builds the repository.
func NewDefinitionRepository(db *mongo.Database) *DefinitionRepository {
	return &DefinitionRepository{coll: db.Collection("custom_attribute_definitions")}
}

func (r *DefinitionRepository) Create(ctx context.Context, d *entity.Definition) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toModel(d))
	return mongodb.MapError(err)
}

func (r *DefinitionRepository) Update(ctx context.Context, d *entity.Definition) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": d.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"label":       d.Label,
			"description": d.Description,
			"options":     d.Options,
			"regex":       d.Regex,
			"updated_at":  d.UpdatedAt,
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

func (r *DefinitionRepository) Delete(ctx context.Context, id string) error {
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

func (r *DefinitionRepository) FindByID(ctx context.Context, id string) (*entity.Definition, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.CustomAttributeDefinition
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *DefinitionRepository) FindByKey(ctx context.Context, appliesTo entity.AppliesTo, key string) (*entity.Definition, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.CustomAttributeDefinition
	filter := bson.M{"tenant_id": tenantID, "applies_to": string(appliesTo), "key": key}
	if err := r.coll.FindOne(ctx, filter).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *DefinitionRepository) List(ctx context.Context, appliesTo entity.AppliesTo, page shared.PageRequest) ([]*entity.Definition, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	base := bson.M{"tenant_id": tenantID}
	if appliesTo.Valid() {
		base["applies_to"] = string(appliesTo)
	}
	filter := mongodb.ApplyKeyset(base, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	return r.query(ctx, filter, opts)
}

func (r *DefinitionRepository) ListAllByAppliesTo(ctx context.Context, appliesTo entity.AppliesTo) ([]*entity.Definition, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	return r.query(ctx, bson.M{"tenant_id": tenantID, "applies_to": string(appliesTo)}, nil)
}

func (r *DefinitionRepository) query(ctx context.Context, filter bson.M, opts *options.FindOptions) ([]*entity.Definition, error) {
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.Definition
	for c.Next(ctx) {
		var m models.CustomAttributeDefinition
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toModel(d *entity.Definition) models.CustomAttributeDefinition {
	m := models.CustomAttributeDefinition{
		Key: d.Key, Label: d.Label, Description: d.Description,
		Type: string(d.Type), AppliesTo: string(d.AppliesTo), Options: d.Options, Regex: d.Regex,
	}
	m.ID = d.ID
	m.TenantID = d.TenantID
	m.CreatedAt = d.CreatedAt
	m.UpdatedAt = d.UpdatedAt
	return m
}

func toEntity(m *models.CustomAttributeDefinition) *entity.Definition {
	return &entity.Definition{
		ID:          m.ID,
		TenantID:    m.TenantID,
		Key:         m.Key,
		Label:       m.Label,
		Description: m.Description,
		Type:        entity.AttributeType(m.Type),
		AppliesTo:   entity.AppliesTo(m.AppliesTo),
		Options:     m.Options,
		Regex:       m.Regex,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

var _ repository.DefinitionRepository = (*DefinitionRepository)(nil)
