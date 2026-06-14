package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0030 adds a partial UNIQUE index on tenants.external_ref so the platform
// provisioner's natural key never produces two tenants. The index is partial
// (only documents that have the field) so self-service signup tenants — which
// carry no external_ref — are unaffected and never collide. Idempotent.
func init() {
	Register(Migration{
		Version: 30,
		Name:    "tenant_external_ref_index",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("tenants").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys: bson.D{{Key: "external_ref", Value: 1}},
				Options: options.Index().
					SetName("external_ref_unique").
					SetUnique(true).
					SetPartialFilterExpression(bson.M{"external_ref": bson.M{"$exists": true}}),
			})
			return err
		},
	})
}
