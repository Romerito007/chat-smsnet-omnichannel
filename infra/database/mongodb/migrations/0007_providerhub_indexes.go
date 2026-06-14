package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0007 adds indexes for the providerhub query log. Idempotent.
func init() {
	Register(Migration{
		Version: 7,
		Name:    "providerhub_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			// Query log: keyset + per-conversation lookup. Kept lean (no payload).
			if _, err := db.Collection("provider_query_logs").Indexes().CreateMany(ctx, []mongo.IndexModel{
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
