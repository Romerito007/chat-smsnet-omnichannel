package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0027 adds the covering index for GET /v1/contacts?tag_id= (filter contacts by
// tag). The list filters by {tenant_id, tags} (exact membership in the tags
// array) and orders by the keyset {created_at desc, _id desc}; this index covers
// both, so the tag filter paginates without a collection scan.
//
// The name/phone filters are case-insensitive substring matches, which a B-tree
// index cannot accelerate (only anchored prefixes can); those run within the
// tenant bound of the existing tenant_keyset index, so no new index applies.
// Idempotent.
func init() {
	Register(Migration{
		Version: 27,
		Name:    "contacts_tag_keyset_index",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("contacts").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys: bson.D{
					{Key: "tenant_id", Value: 1},
					{Key: "tags", Value: 1},
					{Key: "created_at", Value: -1},
					{Key: "_id", Value: -1},
				},
				Options: options.Index().SetName("tenant_tags_keyset"),
			})
			return err
		},
	})
}
