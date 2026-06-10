package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0003 adds indexes for sectors and queues. Idempotent.
func init() {
	Register(Migration{
		Version: 3,
		Name:    "sectors_queues_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			// sectors: unique name per tenant + keyset pagination.
			if _, err := db.Collection("sectors").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "name", Value: 1}},
					Options: options.Index().SetUnique(true).SetName("uniq_tenant_sector_name"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
			}); err != nil {
				return err
			}

			// queues: unique name per tenant, per-sector lookup, keyset.
			if _, err := db.Collection("queues").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "name", Value: 1}},
					Options: options.Index().SetUnique(true).SetName("uniq_tenant_queue_name"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "sector_id", Value: 1}},
					Options: options.Index().SetName("tenant_sector"),
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
