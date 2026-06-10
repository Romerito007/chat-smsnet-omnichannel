package start_routines

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/app/config"
	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// defaultRolePermissions defines the seeded baseline roles and their base
// permissions. Owner gets the wildcard; the others get a conservative subset.
var defaultRolePermissions = map[authz.Role][]string{
	authz.RoleOwner: {"*"},
	authz.RoleAdmin: {
		"conversation:read", "conversation:write",
		"contact:read", "contact:write",
		"channel:read", "channel:write",
		"user:read", "user:write",
		"report:read",
	},
	authz.RoleAgent: {
		"conversation:read", "conversation:write",
		"contact:read",
	},
}

// bootstrapSeeds idempotently creates the first tenant, its default roles and
// the owner user. Re-running is a no-op: every write is an upsert keyed by a
// natural identifier (tenant name, role name, owner email).
func bootstrapSeeds(ctx context.Context, c *container.Container) error {
	now := time.Now().UTC()
	db := c.Mongo.DB

	tenantID, err := upsertTenant(ctx, db.Collection("tenants"), c.Config.Seed.TenantName, now)
	if err != nil {
		return fmt.Errorf("seed tenant: %w", err)
	}

	for role, perms := range defaultRolePermissions {
		if err := upsertRole(ctx, db.Collection("roles"), tenantID, string(role), perms, now); err != nil {
			return fmt.Errorf("seed role %s: %w", role, err)
		}
	}

	if err := upsertOwner(ctx, db.Collection("users"), tenantID, c.Config.Seed, now); err != nil {
		return fmt.Errorf("seed owner: %w", err)
	}

	c.Logger.Info("seed applied", "tenant_id", tenantID, "owner_email", c.Config.Seed.OwnerEmail)
	return nil
}

// upsertTenant inserts the tenant on first run and returns its id, generating a
// new id only when inserting.
func upsertTenant(ctx context.Context, coll *mongo.Collection, name string, now time.Time) (string, error) {
	filter := bson.M{"name": name}
	update := bson.M{
		"$setOnInsert": bson.M{
			"_id":        shared.NewID(),
			"name":       name,
			"created_at": now,
		},
		"$set": bson.M{"updated_at": now},
	}
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After).
		SetProjection(bson.M{"_id": 1})

	var doc struct {
		ID string `bson:"_id"`
	}
	if err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc); err != nil {
		return "", err
	}
	return doc.ID, nil
}

func upsertRole(ctx context.Context, coll *mongo.Collection, tenantID, name string, perms []string, now time.Time) error {
	filter := bson.M{"tenant_id": tenantID, "name": name}
	update := bson.M{
		"$setOnInsert": bson.M{
			"_id":        shared.NewID(),
			"tenant_id":  tenantID,
			"name":       name,
			"created_at": now,
		},
		"$set": bson.M{
			"permissions": perms,
			"updated_at":  now,
		},
	}
	_, err := coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

func upsertOwner(ctx context.Context, coll *mongo.Collection, tenantID string, seed config.SeedConfig, now time.Time) error {
	filter := bson.M{"tenant_id": tenantID, "email": seed.OwnerEmail}
	update := bson.M{
		"$setOnInsert": bson.M{
			"_id":        shared.NewID(),
			"tenant_id":  tenantID,
			"email":      seed.OwnerEmail,
			"created_at": now,
		},
		"$set": bson.M{
			"name":       seed.OwnerName,
			"roles":      []string{string(authz.RoleOwner)},
			"updated_at": now,
		},
	}
	_, err := coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}
