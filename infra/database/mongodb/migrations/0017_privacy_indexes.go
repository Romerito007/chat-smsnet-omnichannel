package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0017 adds indexes for the privacy (LGPD) domain: export-request lookups, the
// retention policy (one per tenant) and the legal-hold lookup used to exempt
// contacts from anonymization/retention. Idempotent.
func init() {
	Register(Migration{
		Version: 17,
		Name:    "privacy_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			// privacy_exports: tenant keyset listing + per-contact lookup.
			if _, err := db.Collection("privacy_exports").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_keyset"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "contact_id", Value: 1}},
					Options: options.Index().SetName("tenant_contact"),
				},
			}); err != nil {
				return err
			}

			// retention_policies: _id is the tenant id (one policy per tenant), so no
			// extra index is required.

			// legal_holds: active-hold lookup by contact.
			if _, err := db.Collection("legal_holds").Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "contact_id", Value: 1}, {Key: "until", Value: 1}},
				Options: options.Index().SetName("tenant_contact_until"),
			}); err != nil {
				return err
			}

			return nil
		},
	})
}
