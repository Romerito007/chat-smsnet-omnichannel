package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0033 indexes webhook subscriptions by (tenant_id, owned_by_channel_id) so a
// channel's managed subscription is located in one query when syncing/removing it.
// The index is partial (only documents that carry an owner) since manual
// subscriptions leave it empty. Idempotent.
func init() {
	Register(Migration{
		Version: 33,
		Name:    "webhook_owned_by_channel_index",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("webhook_subscriptions").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "owned_by_channel_id", Value: 1}},
				Options: options.Index().
					SetName("tenant_owned_by_channel").
					SetPartialFilterExpression(bson.M{"owned_by_channel_id": bson.M{"$exists": true}}),
			})
			return err
		},
	})
}
