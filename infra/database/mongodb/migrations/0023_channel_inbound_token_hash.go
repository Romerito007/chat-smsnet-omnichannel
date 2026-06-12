package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0023 moves the channel integration token from a plaintext field
// (webhook_verify_token) to a hashed-at-rest field (inbound_token_hash): it drops
// the old unique index and creates a unique partial index on the hash (partial so
// pre-existing rows without a hash don't collide on the empty string). Idempotent.
func init() {
	Register(Migration{
		Version: 23,
		Name:    "channel_inbound_token_hash",
		Up: func(ctx context.Context, db *mongo.Database) error {
			coll := db.Collection("channel_connections")
			// Drop the stale plaintext-token index; ignore "index not found".
			_, _ = coll.Indexes().DropOne(ctx, "uniq_webhook_verify_token")

			_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys: bson.D{{Key: "inbound_token_hash", Value: 1}},
				Options: options.Index().
					SetUnique(true).
					SetName("uniq_inbound_token_hash").
					SetPartialFilterExpression(bson.M{"inbound_token_hash": bson.M{"$gt": ""}}),
			})
			return err
		},
	})
}
