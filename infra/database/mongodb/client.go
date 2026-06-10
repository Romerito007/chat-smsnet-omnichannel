// Package mongodb owns the MongoDB driver: connection, the shared *mongo.Database
// handle, BSON models, migrations and concrete repository implementations.
package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/romerito007/chat-smsnet-omnichannel/app/config"
)

// Client wraps the mongo client and the resolved database handle so the rest of
// the app depends on a single, narrow type.
type Client struct {
	mongo *mongo.Client
	DB    *mongo.Database
}

// Connect dials MongoDB and verifies the connection with a ping.
func Connect(ctx context.Context, cfg config.MongoConfig) (*Client, error) {
	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	opts := options.Client().
		ApplyURI(cfg.URI).
		SetServerSelectionTimeout(10 * time.Second)

	cli, err := mongo.Connect(connectCtx, opts)
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	if err := cli.Ping(connectCtx, readpref.Primary()); err != nil {
		_ = cli.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	return &Client{mongo: cli, DB: cli.Database(cfg.Database)}, nil
}

// Ping verifies connectivity; used by health checks.
func (c *Client) Ping(ctx context.Context) error {
	return c.mongo.Ping(ctx, readpref.Primary())
}

// Disconnect closes the underlying connection.
func (c *Client) Disconnect(ctx context.Context) error {
	return c.mongo.Disconnect(ctx)
}
