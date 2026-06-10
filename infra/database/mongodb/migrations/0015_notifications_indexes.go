package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0015 adds indexes for notifications and preferences. Idempotent.
func init() {
	Register(Migration{
		Version: 15,
		Name:    "notifications_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			if _, err := db.Collection("notifications").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					// Inbox: a user's notifications, newest first.
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_user_keyset"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}, {Key: "read", Value: 1}},
					Options: options.Index().SetName("tenant_user_read"),
				},
			}); err != nil {
				return err
			}

			if _, err := db.Collection("notification_preferences").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}},
				Options: options.Index().SetName("tenant_user_unique").SetUnique(true),
			}); err != nil {
				return err
			}
			return nil
		},
	})
}
