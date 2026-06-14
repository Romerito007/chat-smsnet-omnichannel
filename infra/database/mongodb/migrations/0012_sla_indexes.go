package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0012 adds indexes for SLA policies and trackings. Idempotent.
func init() {
	Register(Migration{
		Version: 12,
		Name:    "sla_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("sla_policies").Indexes().CreateMany(ctx, []mongo.IndexModel{
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

			if _, err := db.Collection("sla_trackings").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "conversation_id", Value: 1}},
					Options: options.Index().SetName("tenant_conversation_unique").SetUnique(true),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "status", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_status_keyset"),
				},
				{
					// Used by the cross-tenant sla.check scheduler scan.
					Keys:    bson.D{{Key: "status", Value: 1}},
					Options: options.Index().SetName("status"),
				},
			}); err != nil {
				return err
			}
			return nil
		},
	})
}
