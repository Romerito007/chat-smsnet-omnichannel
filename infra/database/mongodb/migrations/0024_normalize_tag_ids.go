package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// 0024 normalizes tag arrays to canonical ids: conversations.tags and
// contacts.tags historically mixed tag NAMES (from the seed) with tag IDS (from
// the front), which broke the front render and removal. For every tenant tag, any
// occurrence of its NAME in conversations/contacts tags is replaced by its ID
// (ids are left untouched). Idempotent: re-running finds nothing left to replace.
func init() {
	Register(Migration{
		Version: 24,
		Name:    "normalize_tag_ids",
		Up: func(ctx context.Context, db *mongo.Database) error {
			cur, err := db.Collection("tags").Find(ctx, bson.M{})
			if err != nil {
				return err
			}
			defer func() { _ = cur.Close(ctx) }()

			for cur.Next(ctx) {
				var tag struct {
					ID       string `bson:"_id"`
					TenantID string `bson:"tenant_id"`
					Name     string `bson:"name"`
				}
				if err := cur.Decode(&tag); err != nil {
					return err
				}
				if tag.Name == "" || tag.Name == tag.ID {
					continue
				}
				// Replace the exact element "name" with the id inside the tags array.
				replace := mongo.Pipeline{{{Key: "$set", Value: bson.M{"tags": bson.M{"$map": bson.M{
					"input": "$tags",
					"in":    bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$$this", tag.Name}}, tag.ID, "$$this"}},
				}}}}}}
				filter := bson.M{"tenant_id": tag.TenantID, "tags": tag.Name}
				if _, err := db.Collection("conversations").UpdateMany(ctx, filter, replace); err != nil {
					return err
				}
				if _, err := db.Collection("contacts").UpdateMany(ctx, filter, replace); err != nil {
					return err
				}
			}
			return cur.Err()
		},
	})
}
