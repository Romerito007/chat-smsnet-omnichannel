package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0027 adds indexes for the automation_rules collection: a tenant listing index,
// the enabled-by-event resolution index used by the async evaluator, and an
// actions.webhook_id index for the referential-integrity check that blocks
// deleting a webhook a rule references. Idempotent.
func init() {
	Register(Migration{
		Version: 27,
		Name:    "automation_rules_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("automation_rules").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
					Options: options.Index().SetName("tenant_id"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "enabled", Value: 1}, {Key: "event", Value: 1}},
					Options: options.Index().SetName("tenant_enabled_event"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "actions.webhook_id", Value: 1}},
					Options: options.Index().SetName("tenant_action_webhook"),
				},
			})
			return err
		},
	})
}
