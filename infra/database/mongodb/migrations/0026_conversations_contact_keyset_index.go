package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0026 adds the covering index for GET /v1/conversations?contact_id= (the
// contact's conversation history). The list filters by {tenant_id, contact_id}
// and orders by the inbox keyset {updated_at desc, _id desc}. The existing
// tenant_contact_created index (…, created_at) covers the equality but not this
// ordering, forcing an in-memory sort; this index covers both so the contact
// history paginates without a collection scan. Idempotent.
func init() {
	Register(Migration{
		Version: 26,
		Name:    "conversations_contact_keyset_index",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("conversations").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys: bson.D{
					{Key: "tenant_id", Value: 1},
					{Key: "contact_id", Value: 1},
					{Key: "updated_at", Value: -1},
					{Key: "_id", Value: -1},
				},
				Options: options.Index().SetName("tenant_contact_updated_keyset"),
			})
			return err
		},
	})
}
