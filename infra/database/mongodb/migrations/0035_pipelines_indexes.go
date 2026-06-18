package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0035 creates the pipelines indexes: tenant_id (list the tenant's funnels) and
// (tenant_id, is_default) (resolve the tenant's default funnel for the Kanban).
// Idempotent.
func init() {
	Register(Migration{
		Version: 35,
		Name:    "pipelines_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("pipelines").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}},
					Options: options.Index().SetName("tenant_id"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "is_default", Value: -1}},
					Options: options.Index().SetName("tenant_default"),
				},
			})
			return err
		},
	})
}
