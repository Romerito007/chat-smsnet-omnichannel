package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0035 moves the copilot_assistants channel resolution index from the channel
// TYPE to the specific channel CONNECTION id: assistants now serve specific
// channels (channel_ids), not types. It creates the channel_ids index and drops
// the old channel_types one (best-effort — ignored if absent). Idempotent.
func init() {
	Register(Migration{
		Version: 35,
		Name:    "copilot_assistants_channel_ids_index",
		Up: func(ctx context.Context, db *mongo.Database) error {
			coll := db.Collection("copilot_assistants")
			if _, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "enabled", Value: 1}, {Key: "channel_ids", Value: 1}},
				Options: options.Index().SetName("tenant_channel_id"),
			}); err != nil {
				return err
			}
			// Drop the now-unused type index; ignore "index not found".
			_, _ = coll.Indexes().DropOne(ctx, "tenant_channel_type")
			return nil
		},
	})
}
