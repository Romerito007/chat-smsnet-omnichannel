// Package health aggregates liveness/readiness checks for the process's
// dependencies (MongoDB, Redis).
package health

import (
	"context"
	"time"

	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/redis"
)

// Status is the overall health verdict.
type Status string

const (
	StatusOK       Status = "ok"
	StatusDegraded Status = "degraded"
)

// Report is the structured health response.
type Report struct {
	Status Status            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// Checker runs dependency probes.
type Checker struct {
	mongo *mongodb.Client
	redis redis.Client
}

// NewChecker builds a checker over the given dependencies. Any may be nil (a
// role might not use it), in which case it is skipped.
func NewChecker(mongo *mongodb.Client, rdb redis.Client) *Checker {
	return &Checker{mongo: mongo, redis: rdb}
}

// Check probes every configured dependency and returns an aggregate report.
func (c *Checker) Check(ctx context.Context) Report {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	report := Report{Status: StatusOK, Checks: map[string]string{}}

	if c.mongo != nil {
		if err := c.mongo.Ping(ctx); err != nil {
			report.Checks["mongo"] = "error: " + err.Error()
			report.Status = StatusDegraded
		} else {
			report.Checks["mongo"] = "ok"
		}
	}
	if c.redis != nil {
		if err := c.redis.Ping(ctx).Err(); err != nil {
			report.Checks["redis"] = "error: " + err.Error()
			report.Status = StatusDegraded
		} else {
			report.Checks["redis"] = "ok"
		}
	}
	return report
}
