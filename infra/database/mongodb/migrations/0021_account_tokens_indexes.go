package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0021 adds indexes for the account-lifecycle tokens: email verification,
// password reset and invitations. Each has a unique token hash and a TTL on the
// expiry so spent/expired records are reaped automatically. Idempotent.
func init() {
	Register(Migration{
		Version: 21,
		Name:    "account_tokens_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			hashTTL := func(coll string, extra ...mongo.IndexModel) error {
				models := append([]mongo.IndexModel{
					{
						Keys:    bson.D{{Key: "token_hash", Value: 1}},
						Options: options.Index().SetUnique(true).SetName("uniq_token_hash"),
					},
					{
						Keys:    bson.D{{Key: "expires_at", Value: 1}},
						Options: options.Index().SetExpireAfterSeconds(0).SetName("ttl_expires_at"),
					},
				}, extra...)
				_, err := db.Collection(coll).Indexes().CreateMany(ctx, models)
				return err
			}

			if err := hashTTL("email_verification_tokens", mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}},
				Options: options.Index().SetName("tenant_user"),
			}); err != nil {
				return err
			}
			if err := hashTTL("password_reset_tokens", mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}},
				Options: options.Index().SetName("tenant_user"),
			}); err != nil {
				return err
			}
			return hashTTL("invitations", mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "email", Value: 1}},
				Options: options.Index().SetName("tenant_email"),
			})
		},
	})
}
