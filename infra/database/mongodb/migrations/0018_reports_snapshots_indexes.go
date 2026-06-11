package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0018 adds the reports_snapshots index (one per tenant + day). Idempotent.
func init() {
	Register(Migration{
		Version: 18,
		Name:    "reports_snapshots_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("reports_snapshots").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "date", Value: -1}},
				Options: options.Index().SetName("tenant_date").SetUnique(true),
			}); err != nil {
				return err
			}
			return nil
		},
	})
}
