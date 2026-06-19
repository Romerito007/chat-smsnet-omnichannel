package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0038 creates the deal_timeline index: (tenant_id, deal_id, created_at desc, _id
// desc) — the keyset feed of a deal's events, most recent first. Idempotent.
func init() {
	Register(Migration{
		Version: 38,
		Name:    "dealtimeline_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("deal_timeline").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys: bson.D{
						{Key: "tenant_id", Value: 1}, {Key: "deal_id", Value: 1},
						{Key: "created_at", Value: -1}, {Key: "_id", Value: -1},
					},
					Options: options.Index().SetName("tenant_deal_created_keyset"),
				},
			})
			return err
		},
	})
}
