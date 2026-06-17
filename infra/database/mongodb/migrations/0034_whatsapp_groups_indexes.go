package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0034 creates the whatsapp_groups indexes: a UNIQUE (tenant_id, group_jid) so the
// gateway sync upserts idempotently (one row per group per tenant); a keyset index
// (tenant_id, created_at, _id) for the management listing; and a TEXT index on
// (name, description) for the screen's search. Idempotent.
func init() {
	Register(Migration{
		Version: 34,
		Name:    "whatsapp_groups_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("whatsapp_groups").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "group_jid", Value: 1}},
					Options: options.Index().SetName("tenant_group_jid_unique").SetUnique(true),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_created_keyset"),
				},
				{
					Keys:    bson.D{{Key: "name", Value: "text"}, {Key: "description", Value: "text"}},
					Options: options.Index().SetName("name_description_text"),
				},
			})
			return err
		},
	})
}
