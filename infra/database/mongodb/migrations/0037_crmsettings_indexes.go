package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0037 creates the crm_settings index: a UNIQUE tenant_id so each tenant has at most
// one CRM-settings document (the per-tenant module toggles). Idempotent.
func init() {
	Register(Migration{
		Version: 37,
		Name:    "crmsettings_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("crm_settings").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}},
					Options: options.Index().SetName("tenant_id_unique").SetUnique(true),
				},
			})
			return err
		},
	})
}
