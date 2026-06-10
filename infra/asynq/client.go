package asynq

import (
	"github.com/hibiken/asynq"

	"github.com/romerito007/chat-smsnet-omnichannel/app/config"
)

// RedisOpt converts our Redis config into the asynq connection option shared by
// the client, server and scheduler.
func RedisOpt(cfg config.RedisConfig) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}
}

// Client wraps the asynq enqueue client.
type Client struct {
	inner *asynq.Client
}

// NewClient builds an enqueue client.
func NewClient(cfg config.RedisConfig) *Client {
	return &Client{inner: asynq.NewClient(RedisOpt(cfg))}
}

// Enqueue submits a task. Callers pass options such as asynq.Queue(QueueX),
// asynq.MaxRetry(n) or asynq.ProcessIn(d).
func (c *Client) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return c.inner.Enqueue(task, opts...)
}

// Close releases the client's connections.
func (c *Client) Close() error { return c.inner.Close() }
