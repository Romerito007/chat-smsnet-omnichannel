package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0032 adds indexes for custom_attribute_definitions: a UNIQUE index on
// (tenant_id, applies_to, key) so a key is unique within its scope (the same key
// may exist once for contacts and once for conversations), plus a keyset listing
// index per scope. Idempotent.
func init() {
	Register(Migration{
		Version: 32,
		Name:    "custom_attribute_definitions_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("custom_attribute_definitions").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "applies_to", Value: 1}, {Key: "key", Value: 1}},
					Options: options.Index().SetName("tenant_scope_key_unique").SetUnique(true),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "applies_to", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_scope_keyset"),
				},
			})
			return err
		},
	})
}
