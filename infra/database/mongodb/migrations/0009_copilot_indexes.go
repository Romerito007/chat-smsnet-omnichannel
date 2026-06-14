package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0009 adds indexes for the copilot config (one per tenant) and the AI usage
// log. Idempotent.
func init() {
	Register(Migration{
		Version: 9,
		Name:    "copilot_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("copilot_configs").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}},
				Options: options.Index().SetName("tenant_unique").SetUnique(true),
			}); err != nil {
				return err
			}

			if _, err := db.Collection("copilot_logs").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "conversation_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_conversation_keyset"),
				},
			}); err != nil {
				return err
			}

			return nil
		},
	})
}
