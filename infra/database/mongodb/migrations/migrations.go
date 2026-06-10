// Package migrations holds numbered, idempotent migrations that create indexes
// and seed reference data. Each migration registers itself via Register in its
// own NNNN_*.go file; Run executes them in order, recording applied versions in
// a dedicated collection so reruns are no-ops.
package migrations

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// migrationsCollection records which migrations have been applied.
const migrationsCollection = "_migrations"

// Migration is a single, ordered, idempotent schema/seed step.
type Migration struct {
	// Version must be unique and strictly increasing (e.g. 1, 2, 3...).
	Version int
	// Name is a human-readable identifier for logs.
	Name string
	// Up applies the migration. It must be safe to run multiple times.
	Up func(ctx context.Context, db *mongo.Database) error
}

// registry is the package-global ordered set of migrations.
var registry []Migration

// Register adds a migration to the registry. Called from init() in each
// NNNN_*.go file.
func Register(m Migration) {
	registry = append(registry, m)
}

type appliedRecord struct {
	Version   int       `bson:"_id"`
	Name      string    `bson:"name"`
	AppliedAt time.Time `bson:"applied_at"`
}

// Run applies every registered migration not yet recorded, in ascending version
// order. It is safe to call on every boot.
func Run(ctx context.Context, db *mongo.Database) error {
	ordered := make([]Migration, len(registry))
	copy(ordered, registry)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Version < ordered[j].Version })

	coll := db.Collection(migrationsCollection)
	for _, m := range ordered {
		count, err := coll.CountDocuments(ctx, bson.M{"_id": m.Version})
		if err != nil {
			return fmt.Errorf("check migration %d: %w", m.Version, err)
		}
		if count > 0 {
			continue
		}
		if err := m.Up(ctx, db); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", m.Version, m.Name, err)
		}
		rec := appliedRecord{Version: m.Version, Name: m.Name, AppliedAt: time.Now().UTC()}
		if _, err := coll.InsertOne(ctx, rec); err != nil {
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}
	}
	return nil
}
