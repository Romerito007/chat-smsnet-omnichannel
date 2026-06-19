package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0039 creates the deal_tasks indexes: (tenant_id, deal_id) for a deal's tasks;
// (tenant_id, assigned_to, status) for "my pending tasks"; (tenant_id, due_date,
// status) for tasks coming due. Idempotent.
func init() {
	Register(Migration{
		Version: 39,
		Name:    "dealtasks_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("deal_tasks").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "deal_id", Value: 1}},
					Options: options.Index().SetName("tenant_deal"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "assigned_to", Value: 1}, {Key: "status", Value: 1}},
					Options: options.Index().SetName("tenant_assigned_status"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "due_date", Value: 1}, {Key: "status", Value: 1}},
					Options: options.Index().SetName("tenant_due_status"),
				},
			})
			return err
		},
	})
}
