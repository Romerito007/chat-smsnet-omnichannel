package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0017 adds the search indexes: a text index on contacts (name/document/phone)
// and on messages (text), plus supporting keyset indexes. Idempotent.
func init() {
	Register(Migration{
		Version: 17,
		Name:    "search_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			// Contacts free-text search.
			if _, err := db.Collection("contacts").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys: bson.D{
					{Key: "name", Value: "text"},
					{Key: "document", Value: "text"},
					{Key: "phone", Value: "text"},
				},
				Options: options.Index().SetName("contacts_text"),
			}); err != nil {
				return err
			}

			// Messages free-text search + a keyset for scanning matches by recency.
			if _, err := db.Collection("messages").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "text", Value: "text"}},
				Options: options.Index().SetName("messages_text"),
			}); err != nil {
				return err
			}
			if _, err := db.Collection("messages").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
				Options: options.Index().SetName("tenant_keyset"),
			}); err != nil {
				return err
			}

			// Conversation search supporting index (updated_at keyset already exists
			// from 0004; add a contact lookup for contact scoping).
			if _, err := db.Collection("conversations").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "contact_id", Value: 1}},
				Options: options.Index().SetName("tenant_contact"),
			}); err != nil {
				return err
			}

			return nil
		},
	})
}
