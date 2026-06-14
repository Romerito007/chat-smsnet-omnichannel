package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0028 adds the keyset index for rule_evaluation_logs (the minimal automation-rule
// firing log), used by GET /v1/automation-rules/{id}/logs. Idempotent.
func init() {
	Register(Migration{
		Version: 28,
		Name:    "rule_evaluation_logs_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("rule_evaluation_logs").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "rule_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
				Options: options.Index().SetName("tenant_rule_keyset"),
			})
			return err
		},
	})
}
