package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0001 creates the baseline indexes for the core multi-tenant collections.
// Index creation is idempotent in MongoDB: re-issuing the same index spec is a
// no-op, and the migration runner also guards against re-running.
func init() {
	Register(Migration{
		Version: 1,
		Name:    "baseline_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			// tenants: unique by id (the _id), lookup by name.
			if _, err := db.Collection("tenants").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys: bson.D{{Key: "name", Value: 1}},
			}); err != nil {
				return err
			}

			// users: unique email per tenant.
			if _, err := db.Collection("users").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "email", Value: 1}},
					Options: options.Index().SetUnique(true).SetName("uniq_tenant_email"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
			}); err != nil {
				return err
			}

			// audit log: keyset pagination per tenant.
			if _, err := db.Collection("audit_logs").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
				Options: options.Index().SetName("tenant_keyset"),
			}); err != nil {
				return err
			}

			return nil
		},
	})
}
