package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0008 adds indexes for webhook subscriptions and deliveries. Idempotent.
func init() {
	Register(Migration{
		Version: 8,
		Name:    "webhook_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("webhook_subscriptions").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					// Keyset listing of a tenant's webhooks.
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
				{
					// Dispatcher lookup: enabled subscriptions for a tenant + event.
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "enabled", Value: 1}, {Key: "events", Value: 1}},
					Options: options.Index().SetName("tenant_enabled_events"),
				},
			}); err != nil {
				return err
			}

			if _, err := db.Collection("webhook_deliveries").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					// Delivery history for one webhook, newest first.
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "webhook_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_webhook_keyset"),
				},
			}); err != nil {
				return err
			}

			return nil
		},
	})
}
