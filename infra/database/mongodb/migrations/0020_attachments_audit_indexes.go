package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0020 adds the final MVP indexes: attachment lookups (by conversation and by
// message) and an audit-log index by action so the audit.view query can filter
// by action prefix efficiently. Idempotent.
func init() {
	Register(Migration{
		Version: 20,
		Name:    "attachments_audit_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("attachments").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "conversation_id", Value: 1}, {Key: "created_at", Value: -1}},
					Options: options.Index().SetName("tenant_conversation_created"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "message_id", Value: 1}},
					Options: options.Index().SetName("tenant_message"),
				},
			}); err != nil {
				return err
			}

			// audit_logs: filter by action within a tenant (newest first).
			if _, err := db.Collection("audit_logs").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "action", Value: 1}, {Key: "created_at", Value: -1}},
				Options: options.Index().SetName("tenant_action_created"),
			}); err != nil {
				return err
			}

			return nil
		},
	})
}
