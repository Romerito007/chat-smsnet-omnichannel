package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0004 adds indexes for conversations, messages and conversation events.
// Idempotent.
func init() {
	Register(Migration{
		Version: 4,
		Name:    "conversations_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("conversations").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "status", Value: 1}, {Key: "sector_id", Value: 1}, {Key: "updated_at", Value: -1}},
					Options: options.Index().SetName("tenant_status_sector_updated"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "assigned_to", Value: 1}, {Key: "status", Value: 1}},
					Options: options.Index().SetName("tenant_assignee_status"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "contact_id", Value: 1}, {Key: "created_at", Value: -1}},
					Options: options.Index().SetName("tenant_contact_created"),
				},
				{
					// Inbox keyset ordering.
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "updated_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_updated_keyset"),
				},
			}); err != nil {
				return err
			}

			if _, err := db.Collection("messages").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "conversation_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
				Options: options.Index().SetName("tenant_conversation_created"),
			}); err != nil {
				return err
			}

			if _, err := db.Collection("conversation_events").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "conversation_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
				Options: options.Index().SetName("tenant_conversation_created"),
			}); err != nil {
				return err
			}

			return nil
		},
	})
}
