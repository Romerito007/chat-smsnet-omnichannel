package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// 0037 moves holidays from sector scope to channel scope. Global holidays
// (scope "all_sectors") become "all_channels" — they stay global and valid.
// Sector-scoped holidays (scope "sectors") referenced sector ids, which no
// longer gate holidays and do not map to channels, so they are removed (the
// system is in development; recreate them as channel-scoped). The obsolete
// sector_ids field is dropped from any remaining document. Idempotent.
func init() {
	Register(Migration{
		Version: 37,
		Name:    "holiday_scope_sector_to_channel",
		Up: func(ctx context.Context, db *mongo.Database) error {
			holidays := db.Collection("holidays")
			if _, err := holidays.UpdateMany(ctx,
				bson.M{"scope": "all_sectors"},
				bson.M{"$set": bson.M{"scope": "all_channels"}},
			); err != nil {
				return err
			}
			if _, err := holidays.DeleteMany(ctx, bson.M{"scope": "sectors"}); err != nil {
				return err
			}
			_, err := holidays.UpdateMany(ctx,
				bson.M{"sector_ids": bson.M{"$exists": true}},
				bson.M{"$unset": bson.M{"sector_ids": ""}},
			)
			return err
		},
	})
}
