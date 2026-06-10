package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0012 adds indexes for conversationtools (tags, canned responses, close
// reasons). Idempotent.
func init() {
	Register(Migration{
		Version: 12,
		Name:    "conversationtools_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("tags").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
				Options: options.Index().SetName("tenant_keyset"),
			}); err != nil {
				return err
			}

			if _, err := db.Collection("canned_responses").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
				{
					// Shortcut is unique per tenant.
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "shortcut", Value: 1}},
					Options: options.Index().SetName("tenant_shortcut_unique").SetUnique(true),
				},
			}); err != nil {
				return err
			}

			if _, err := db.Collection("close_reasons").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
				Options: options.Index().SetName("tenant_keyset"),
			}); err != nil {
				return err
			}

			return nil
		},
	})
}
