package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0005 adds indexes for contacts, channel integrations and the inbound
// idempotency ledger, plus the conversation lookup used by inbound. Idempotent.
func init() {
	Register(Migration{
		Version: 5,
		Name:    "channels_contacts_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			// contacts: identity lookup + keyset.
			if _, err := db.Collection("contacts").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "identities.channel", Value: 1}, {Key: "identities.external_id", Value: 1}},
					Options: options.Index().SetName("tenant_channel_identity"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
			}); err != nil {
				return err
			}

			// channel_integrations: unique public key + keyset.
			if _, err := db.Collection("channel_integrations").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "integration_key", Value: 1}},
					Options: options.Index().SetUnique(true).SetName("uniq_integration_key"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
			}); err != nil {
				return err
			}

			// inbound_messages: idempotency uniqueness.
			if _, err := db.Collection("inbound_messages").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "channel", Value: 1}, {Key: "external_message_id", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("uniq_tenant_channel_external_msg"),
			}); err != nil {
				return err
			}

			// conversations: open-conversation lookup by contact + channel.
			if _, err := db.Collection("conversations").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "contact_id", Value: 1}, {Key: "channel", Value: 1}, {Key: "status", Value: 1}},
				Options: options.Index().SetName("tenant_contact_channel_status"),
			}); err != nil {
				return err
			}

			return nil
		},
	})
}
