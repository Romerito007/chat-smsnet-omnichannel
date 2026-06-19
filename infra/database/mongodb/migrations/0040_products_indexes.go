package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0040 creates the products indexes: (tenant_id, active) for the active-catalog
// listing and (tenant_id, name) for name search, plus a keyset index for the listing.
// Idempotent.
func init() {
	Register(Migration{
		Version: 40,
		Name:    "products_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("products").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "active", Value: 1}},
					Options: options.Index().SetName("tenant_active"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "name", Value: 1}},
					Options: options.Index().SetName("tenant_name"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_created_keyset"),
				},
			})
			return err
		},
	})
}
