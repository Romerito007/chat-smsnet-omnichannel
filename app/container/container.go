// Package container is the composition root: it constructs and holds the shared
// infrastructure dependencies (logger, Mongo, Redis, Asynq, realtime) that the
// factories and start routines draw from. It is built once per process.
package container

import (
	"context"
	"fmt"

	"github.com/romerito007/chat-smsnet-omnichannel/app/config"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	infraasynq "github.com/romerito007/chat-smsnet-omnichannel/infra/asynq"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/realtime"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/redis"
)

// Container holds process-wide singletons. Not every field is populated for
// every role, but the common dependencies (Mongo, Redis) are shared by all.
type Container struct {
	Config config.Config
	Logger shared.Logger

	Mongo *mongodb.Client
	Redis redis.Client

	AsynqClient *infraasynq.Client
	Realtime    *realtime.Manager
}

// New builds the container, connecting to the infrastructure required by the
// active role. Connections are established eagerly so a misconfiguration fails
// fast at startup.
func New(ctx context.Context, cfg config.Config) (*Container, error) {
	logger := shared.NewLogger(cfg.LogLevel)

	mongoClient, err := mongodb.Connect(ctx, cfg.Mongo)
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}

	redisClient, err := redis.Connect(ctx, cfg.Redis)
	if err != nil {
		_ = mongoClient.Disconnect(context.Background())
		return nil, fmt.Errorf("connect redis: %w", err)
	}

	c := &Container{
		Config:      cfg,
		Logger:      logger,
		Mongo:       mongoClient,
		Redis:       redisClient,
		AsynqClient: infraasynq.NewClient(cfg.Redis),
		Realtime:    realtime.NewManager(redisClient),
	}
	return c, nil
}

// Close releases all held resources in reverse order of acquisition.
func (c *Container) Close(ctx context.Context) {
	if c.AsynqClient != nil {
		_ = c.AsynqClient.Close()
	}
	if c.Redis != nil {
		_ = c.Redis.Close()
	}
	if c.Mongo != nil {
		_ = c.Mongo.Disconnect(ctx)
	}
}
