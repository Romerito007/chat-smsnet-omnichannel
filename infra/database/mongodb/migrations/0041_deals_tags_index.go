package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0041 adds a (tenant_id, tags) multikey index so the Kanban can filter deals by an
// etiqueta (GET /v1/deals?tag_id=). Idempotent.
func init() {
	Register(Migration{
		Version: 41,
		Name:    "deals_tags_index",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("deals").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "tags", Value: 1}},
					Options: options.Index().SetName("tenant_tags"),
				},
			})
			return err
		},
	})
}
