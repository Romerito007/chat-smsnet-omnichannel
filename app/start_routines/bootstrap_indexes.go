package start_routines

import (
	"context"
	"fmt"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/migrations"
)

// bootstrapIndexes runs the numbered, idempotent migrations that create indexes
// (and any reference data registered as a migration). Safe to run on every boot.
func bootstrapIndexes(ctx context.Context, c *container.Container) error {
	if err := migrations.Run(ctx, c.Mongo.DB); err != nil {
		return fmt.Errorf("bootstrap indexes: %w", err)
	}
	c.Logger.Info("migrations applied")
	return nil
}
