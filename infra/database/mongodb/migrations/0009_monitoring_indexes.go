package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0009 adds indexes for monitoring config and the query log. Idempotent.
func init() {
	Register(Migration{
		Version: 9,
		Name:    "monitoring_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("monitoring_configs").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "enabled", Value: 1}},
				Options: options.Index().SetName("tenant_enabled"),
			}); err != nil {
				return err
			}

			if _, err := db.Collection("monitoring_query_logs").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "conversation_id", Value: 1}, {Key: "created_at", Value: -1}},
					Options: options.Index().SetName("tenant_conversation"),
				},
			}); err != nil {
				return err
			}

			return nil
		},
	})
}
