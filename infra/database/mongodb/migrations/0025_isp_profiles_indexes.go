package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0025 adds indexes for the new isp_profiles collection (multiple ISP profiles
// per tenant). The partial-unique index guarantees at most one default profile
// per tenant at the database level. Idempotent.
func init() {
	Register(Migration{
		Version: 25,
		Name:    "isp_profiles_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("isp_profiles").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
					Options: options.Index().SetName("tenant_id"),
				},
				{
					// At most one default per tenant: unique on tenant_id, but only
					// over documents where is_default is true.
					Keys: bson.D{{Key: "tenant_id", Value: 1}},
					Options: options.Index().
						SetName("uniq_tenant_default").
						SetUnique(true).
						SetPartialFilterExpression(bson.M{"is_default": true}),
				},
			})
			return err
		},
	})
}
