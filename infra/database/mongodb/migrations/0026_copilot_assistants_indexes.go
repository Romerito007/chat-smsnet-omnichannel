package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0026 adds indexes for copilot_assistants (many per tenant): a tenant listing
// index, a channel resolution index (enabled assistant serving a SPECIFIC channel
// connection by id), and an isp_profile_id index for the referential-integrity
// check on profile delete. Idempotent.
func init() {
	Register(Migration{
		Version: 26,
		Name:    "copilot_assistants_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("copilot_assistants").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
					Options: options.Index().SetName("tenant_id"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "enabled", Value: 1}, {Key: "channel_ids", Value: 1}},
					Options: options.Index().SetName("tenant_channel_id"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "isp_profile_id", Value: 1}},
					Options: options.Index().SetName("tenant_isp_profile"),
				},
			})
			return err
		},
	})
}
