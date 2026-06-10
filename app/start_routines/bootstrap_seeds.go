package start_routines

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/authz"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// bootstrapSeeds idempotently creates the first tenant, its default roles
// (owner/admin/agent with their permissions) and the owner user with a hashed
// password. Re-running is a no-op: every write is an upsert keyed by a natural
// identifier (tenant name, role name, owner email).
func bootstrapSeeds(ctx context.Context, c *container.Container) error {
	now := time.Now().UTC()
	db := c.Mongo.DB

	tenantID, err := upsertTenant(ctx, db.Collection("tenants"), c.Config.Seed.TenantName, now)
	if err != nil {
		return fmt.Errorf("seed tenant: %w", err)
	}

	var ownerRoleID string
	for _, def := range authz.DefaultRoles() {
		roleID, err := upsertRole(ctx, db.Collection("roles"), tenantID, def, now)
		if err != nil {
			return fmt.Errorf("seed role %s: %w", def.Name, err)
		}
		if def.Name == authz.DefaultRoleOwner {
			ownerRoleID = roleID
		}
	}

	if err := upsertOwner(ctx, db.Collection("users"), c, tenantID, ownerRoleID, now); err != nil {
		return fmt.Errorf("seed owner: %w", err)
	}

	c.Logger.Info("seed applied", "tenant_id", tenantID, "owner_email", c.Config.Seed.OwnerEmail)
	return nil
}

// upsertTenant inserts the tenant on first run and returns its id.
func upsertTenant(ctx context.Context, coll *mongo.Collection, name string, now time.Time) (string, error) {
	filter := bson.M{"name": name}
	update := bson.M{
		"$setOnInsert": bson.M{
			"_id":        shared.NewID(),
			"name":       name,
			"status":     "active",
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

// upsertRole upserts a default role and returns its id.
func upsertRole(ctx context.Context, coll *mongo.Collection, tenantID string, def authz.DefaultRoleDefinition, now time.Time) (string, error) {
	perms := make([]string, len(def.Permissions))
	for i, p := range def.Permissions {
		perms[i] = string(p)
	}
	filter := bson.M{"tenant_id": tenantID, "name": def.Name}
	update := bson.M{
		"$setOnInsert": bson.M{
			"_id":        shared.NewID(),
			"tenant_id":  tenantID,
			"name":       def.Name,
			"created_at": now,
		},
		"$set": bson.M{
			"permissions":  perms,
			"sector_scope": string(def.SectorScope),
			"updated_at":   now,
		},
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

// upsertOwner creates the owner user with a hashed password on first run. The
// password is only written on insert, so re-seeding never overwrites a rotated
// password.
func upsertOwner(ctx context.Context, coll *mongo.Collection, c *container.Container, tenantID, ownerRoleID string, now time.Time) error {
	seed := c.Config.Seed

	hash, err := c.Hasher.Hash(seed.OwnerPassword)
	if err != nil {
		return fmt.Errorf("hash owner password: %w", err)
	}

	roleIDs := []string{}
	if ownerRoleID != "" {
		roleIDs = append(roleIDs, ownerRoleID)
	}

	filter := bson.M{"tenant_id": tenantID, "email": seed.OwnerEmail}
	update := bson.M{
		"$setOnInsert": bson.M{
			"_id":                  shared.NewID(),
			"tenant_id":            tenantID,
			"email":                seed.OwnerEmail,
			"password_hash":        hash,
			"status":               "active",
			"role_ids":             roleIDs,
			"sector_ids":           []string{},
			"max_concurrent_chats": 0,
			"created_at":           now,
		},
		"$set": bson.M{
			"name":       seed.OwnerName,
			"updated_at": now,
		},
	}
	_, err = coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}
