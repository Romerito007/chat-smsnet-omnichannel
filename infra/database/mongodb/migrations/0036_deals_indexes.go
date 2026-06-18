package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 0036 creates the deals indexes: (tenant_id, pipeline_id, stage_id) to build the
// Kanban (deals per stage), plus (tenant_id, assigned_to), (tenant_id, contact_id),
// (tenant_id, status) for the seller/contact/status filters, and a keyset index for
// the listing. Idempotent.
func init() {
	Register(Migration{
		Version: 36,
		Name:    "deals_indexes",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("deals").Indexes().CreateMany(ctx, []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "pipeline_id", Value: 1}, {Key: "stage_id", Value: 1}},
					Options: options.Index().SetName("tenant_pipeline_stage"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "assigned_to", Value: 1}},
					Options: options.Index().SetName("tenant_assigned"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "contact_id", Value: 1}},
					Options: options.Index().SetName("tenant_contact"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "status", Value: 1}},
					Options: options.Index().SetName("tenant_status"),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
					Options: options.Index().SetName("tenant_created_keyset"),
				},
			})
			return err
		},
	})
}
