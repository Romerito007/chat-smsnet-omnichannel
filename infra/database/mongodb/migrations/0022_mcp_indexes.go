package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0022 adds indexes for the MCP collections: per-tenant server keyset + enabled
// lookup, approvals by conversation, and call logs by tenant/conversation.
// Idempotent.
func init() {
	Register(Migration{
		Version: 22,
		Name:    "mcp_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("mcp_servers").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "enabled", Value: 1}},
					Options: options.Index().SetName("tenant_enabled"),
				},
			}); err != nil {
				return err
			}
			if _, err := db.Collection("mcp_approvals").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "conversation_id", Value: 1}, {Key: "created_at", Value: -1}},
					Options: options.Index().SetName("tenant_conversation"),
				},
			}); err != nil {
				return err
			}
			_, err := db.Collection("mcp_call_logs").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "conversation_id", Value: 1}, {Key: "created_at", Value: -1}},
					Options: options.Index().SetName("tenant_conversation"),
				},
			})
			return err
		},
	})
}
