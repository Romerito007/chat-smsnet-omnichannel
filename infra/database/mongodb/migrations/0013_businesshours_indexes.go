package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0013 adds indexes for holidays. Idempotent.
func init() {
	Register(Migration{
		Version: 13,
		Name:    "businesshours_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("holidays").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "date", Value: 1}},
					Options: options.Index().SetName("tenant_date"),
				},
			}); err != nil {
				return err
			}
			return nil
		},
	})
}
