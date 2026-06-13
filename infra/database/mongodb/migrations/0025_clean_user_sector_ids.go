package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// 0025 cleans the users.sector_ids representation, which the demo seed had
// corrupted: the team step ran before sectors were created, so agents were saved
// with sector_ids: [""] (a junk empty string, not a real sector id) and the admin
// with null. Both break sector assignment — evaluateAgent checks membership and
// GET /v1/agents?sector_id=X matches on sector_ids, so an agent bound to "" (or
// null) belongs to NO real sector and never matches.
//
// This migration normalizes the representation for ALL users (remove "" entries,
// null -> []) and re-binds the known demo agents to their correct sectors,
// resolving the sector id by name within each agent's tenant. Idempotent.
func init() {
	Register(Migration{
		Version: 25,
		Name:    "clean_user_sector_ids",
		Up: func(ctx context.Context, db *mongo.Database) error {
			users := db.Collection("users")

			// 1) Drop every empty-string entry from sector_ids ([""] -> []).
			if _, err := users.UpdateMany(ctx,
				bson.M{"sector_ids": ""},
				bson.M{"$pull": bson.M{"sector_ids": ""}}); err != nil {
				return err
			}
			// 2) null/missing sector_ids -> [] (consistent "sem setor").
			if _, err := users.UpdateMany(ctx,
				bson.M{"sector_ids": nil},
				bson.M{"$set": bson.M{"sector_ids": bson.A{}}}); err != nil {
				return err
			}

			// 3) Re-bind the demo agents to their correct sector (by name, within
			//    the agent's own tenant) when they currently have no real sector.
			demo := []struct{ email, sector string }{
				{"bruno@demo.local", "Suporte Técnico"},
				{"carla@demo.local", "Suporte Técnico"},
				{"diego@demo.local", "Comercial"},
				{"erica@demo.local", "Financeiro"},
			}
			for _, a := range demo {
				var user struct {
					ID        string   `bson:"_id"`
					TenantID  string   `bson:"tenant_id"`
					SectorIDs []string `bson:"sector_ids"`
				}
				err := users.FindOne(ctx, bson.M{"email": a.email}).Decode(&user)
				if err != nil {
					if err == mongo.ErrNoDocuments {
						continue // demo not seeded in this environment
					}
					return err
				}
				if hasRealSector(user.SectorIDs) {
					continue // already correctly bound
				}
				var sector struct {
					ID string `bson:"_id"`
				}
				err = db.Collection("sectors").FindOne(ctx,
					bson.M{"tenant_id": user.TenantID, "name": a.sector}).Decode(&sector)
				if err != nil {
					if err == mongo.ErrNoDocuments {
						continue // sector absent in this environment
					}
					return err
				}
				if _, err := users.UpdateOne(ctx,
					bson.M{"_id": user.ID},
					bson.M{"$set": bson.M{"sector_ids": bson.A{sector.ID}}}); err != nil {
					return err
				}
			}
			return nil
		},
	})
}

// hasRealSector reports whether the slice holds at least one non-empty sector id.
func hasRealSector(ids []string) bool {
	for _, id := range ids {
		if id != "" {
			return true
		}
	}
	return false
}
