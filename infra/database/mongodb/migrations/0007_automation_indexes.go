package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0007 adds indexes for automation integrations and runs. Idempotent.
func init() {
	Register(Migration{
		Version: 7,
		Name:    "automation_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("automation_integrations").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "enabled", Value: 1}},
					Options: options.Index().SetName("tenant_enabled"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
			}); err != nil {
				return err
			}

			if _, err := db.Collection("automation_runs").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "external_run_id", Value: 1}},
					Options: options.Index().SetName("tenant_external_run"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "conversation_id", Value: 1}, {Key: "created_at", Value: -1}},
					Options: options.Index().SetName("tenant_conversation"),
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
