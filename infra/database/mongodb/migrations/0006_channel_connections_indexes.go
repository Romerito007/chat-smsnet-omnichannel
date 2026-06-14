package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0006 adds indexes for channel connections. Idempotent. (Outbound deliveries are
// no longer a separate rail — message delivery flows through webhooks — so there is
// no outbound_deliveries collection to index.)
func init() {
	Register(Migration{
		Version: 6,
		Name:    "channel_connections_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("channel_connections").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "webhook_verify_token", Value: 1}},
					Options: options.Index().SetUnique(true).SetName("uniq_webhook_verify_token"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "type", Value: 1}, {Key: "enabled", Value: 1}},
					Options: options.Index().SetName("tenant_type_enabled"),
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
