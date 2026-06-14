package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0031 indexes conversations by (tenant_id, protocol) so an agent can locate a
// conversation by its protocol number (GET /v1/conversations?protocol=...). The
// index is partial (only documents that carry a protocol) since single-mode
// channels leave it empty. Idempotent.
func init() {
	Register(Migration{
		Version: 31,
		Name:    "conversations_protocol_index",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("conversations").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "protocol", Value: 1}},
				Options: options.Index().
					SetName("tenant_protocol").
					SetPartialFilterExpression(bson.M{"protocol": bson.M{"$exists": true}}),
			})
			return err
		},
	})
}
