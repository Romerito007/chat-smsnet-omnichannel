package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0028 closes the conversation list index gaps flagged in docs/performance-audit.md
// (§4/§9), so hot list filters no longer sort in memory:
//   - tenant_assignee_updated_keyset: the agent "Minhas" tab
//     (GET /v1/conversations?assigned_to=me) — filter + the updated_at keyset sort.
//   - tenant_tags_updated_keyset: the tag filter (?tag=) — multikey on tags + sort.
//   - tenant_status_last_message: the chat.close_inactive job's ListInactiveOpen,
//     which filters status + last_message_at and sorts last_message_at ascending.
//
// Idempotent: re-creating an identical index is a no-op.
func init() {
	Register(Migration{
		Version: 28,
		Name:    "conversations_list_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("conversations").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys: bson.D{
						{Key: "tenant_id", Value: 1},
						{Key: "assigned_to", Value: 1},
						{Key: "updated_at", Value: -1},
						{Key: "_id", Value: -1},
					},
					Options: options.Index().SetName("tenant_assignee_updated_keyset"),
				},
				{
					Keys: bson.D{
						{Key: "tenant_id", Value: 1},
						{Key: "tags", Value: 1},
						{Key: "updated_at", Value: -1},
						{Key: "_id", Value: -1},
					},
					Options: options.Index().SetName("tenant_tags_updated_keyset"),
				},
				{
					Keys: bson.D{
						{Key: "tenant_id", Value: 1},
						{Key: "status", Value: 1},
						{Key: "last_message_at", Value: 1},
					},
					Options: options.Index().SetName("tenant_status_last_message"),
				},
			})
			return err
		},
	})
}
