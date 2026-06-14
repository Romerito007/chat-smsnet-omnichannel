package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// 0036 removes the business_hours field from every sector document: business
// hours moved to the ChannelConnection, so the sector no longer carries them.
// Idempotent.
func init() {
	Register(Migration{
		Version: 36,
		Name:    "drop_sector_business_hours",
		Up: func(ctx context.Context, db *mongo.Database) error {
			_, err := db.Collection("sectors").UpdateMany(ctx,
				bson.M{"business_hours": bson.M{"$exists": true}},
				bson.M{"$unset": bson.M{"business_hours": ""}},
			)
			return err
		},
	})
}
