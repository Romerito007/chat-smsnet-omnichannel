package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0014 adds indexes for CSAT surveys and responses. Idempotent.
func init() {
	Register(Migration{
		Version: 14,
		Name:    "csat_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("csat_surveys").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "enabled", Value: 1}},
					Options: options.Index().SetName("tenant_enabled"),
				},
			}); err != nil {
				return err
			}

			if _, err := db.Collection("csat_responses").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					// Public token lookup (globally unique).
					Keys:    bson.D{{Key: "token", Value: 1}},
					Options: options.Index().SetName("token_unique").SetUnique(true),
				},
				{
					// One survey per conversation (no re-send).
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "conversation_id", Value: 1}},
					Options: options.Index().SetName("tenant_conversation_unique").SetUnique(true),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
			}); err != nil {
				return err
			}
			return nil
		},
	})
}
