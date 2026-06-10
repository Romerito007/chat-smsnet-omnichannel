package start_routines

import (
	"context"
	"fmt"

	"github.com/romerito007/chat-smsnet-omnichannel/app/container"
)

// bootstrapMongo verifies the MongoDB connection is live before the process
// starts serving. The connection itself is established by the container; this
// routine is the explicit, logged checkpoint the architecture calls for.
func bootstrapMongo(ctx context.Context, c *container.Container) error {
	if err := c.Mongo.Ping(ctx); err != nil {
		return fmt.Errorf("bootstrap mongo: %w", err)
	}
	c.Logger.Info("mongodb ready", "database", c.Config.Mongo.Database)
	return nil
}
