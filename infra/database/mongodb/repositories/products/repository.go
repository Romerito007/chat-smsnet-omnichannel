// Package products is the Mongo implementation of the product-catalog repository.
// Every operation is scoped by the tenant from the context.
package products

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/products/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/products/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/products/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// Repository implements repository.ProductRepository over MongoDB.
type Repository struct {
	coll *mongo.Collection
}

// New builds the repository.
func New(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("products")}
}

func (r *Repository) Create(ctx context.Context, p *entity.Product) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toModel(p))
	return mongodb.MapError(err)
}

func (r *Repository) Update(ctx context.Context, p *entity.Product) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": p.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":        p.Name,
			"description": p.Description,
			"price":       p.Price,
			"currency":    p.Currency,
			"sku":         p.SKU,
			"active":      p.Active,
			"updated_at":  p.UpdatedAt,
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

func (r *Repository) FindByID(ctx context.Context, id string) (*entity.Product, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.Product
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *Repository) List(ctx context.Context, f contracts.ListFilter, page shared.PageRequest) ([]*entity.Product, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	base := bson.M{"tenant_id": tenantID}
	if f.Q != "" {
		base["name"] = bson.M{"$regex": f.Q, "$options": "i"}
	}
	if f.Active != nil {
		base["active"] = *f.Active
	}
	filter := mongodb.ApplyKeyset(base, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.Product
	for c.Next(ctx) {
		var m models.Product
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func toModel(p *entity.Product) models.Product {
	m := models.Product{
		Name: p.Name, Description: p.Description, Price: p.Price,
		Currency: p.Currency, SKU: p.SKU, Active: p.Active,
	}
	m.ID = p.ID
	m.TenantID = p.TenantID
	m.CreatedAt = p.CreatedAt
	m.UpdatedAt = p.UpdatedAt
	return m
}

func toEntity(m *models.Product) *entity.Product {
	return &entity.Product{
		ID: m.ID, TenantID: m.TenantID, Name: m.Name, Description: m.Description,
		Price: m.Price, Currency: m.Currency, SKU: m.SKU, Active: m.Active,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

var _ repository.ProductRepository = (*Repository)(nil)
