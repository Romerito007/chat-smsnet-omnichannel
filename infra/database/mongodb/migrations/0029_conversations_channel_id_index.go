package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0029 indexes conversations by (tenant, contact, channel_id, status) so the
// inbound reuse lookup (FindOpenByContactChannelID) — which now keys by the
// specific channel connection id rather than the channel type — is efficient.
// Idempotent.
func init() {
	Register(Migration{
		Version: 29,
		Name:    "conversations_channel_id_index",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("conversations").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys: bson.D{
					{Key: "tenant_id", Value: 1},
					{Key: "contact_id", Value: 1},
					{Key: "channel_id", Value: 1},
					{Key: "status", Value: 1},
				},
				Options: options.Index().SetName("tenant_contact_channelid_status"),
			})
			return err
		},
	})
}
