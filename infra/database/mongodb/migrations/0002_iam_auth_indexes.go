package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0002 adds indexes for the IAM (roles) and auth (refresh_tokens) collections.
// Idempotent: re-issuing identical index specs is a no-op.
func init() {
	Register(Migration{
		Version: 2,
		Name:    "iam_auth_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			// roles: unique name per tenant + keyset pagination.
			if _, err := db.Collection("roles").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "name", Value: 1}},
					Options: options.Index().SetUnique(true).SetName("uniq_tenant_role_name"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
			}); err != nil {
				return err
			}

			// refresh_tokens: unique hash, per-user lookup, and TTL on expiry.
			if _, err := db.Collection("refresh_tokens").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "token_hash", Value: 1}},
					Options: options.Index().SetUnique(true).SetName("uniq_token_hash"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}},
					Options: options.Index().SetName("tenant_user"),
				},
				{
					Keys:    bson.D{{Key: "expires_at", Value: 1}},
					Options: options.Index().SetExpireAfterSeconds(0).SetName("ttl_expires_at"),
				},
			}); err != nil {
				return err
			}

			return nil
		},
	})
}
