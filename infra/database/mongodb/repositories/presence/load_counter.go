// Package presence holds the Mongo-backed load counter that derives an agent's
// current load from open, assigned conversations. The conversations collection
// is owned by a later domain; counting an absent collection returns 0.
package presence

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/presence/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
)

// openStatuses are the conversation statuses that count toward an agent's load.
var openStatuses = bson.A{"open", "assigned", "pending"}

// LoadCounter implements repository.LoadCounter over the conversations collection.
type LoadCounter struct {
	coll *mongo.Collection
}

// NewLoadCounter builds the counter.
func NewLoadCounter(db *mongo.Database) *LoadCounter {
	return &LoadCounter{coll: db.Collection("conversations")}
}

// CountOpenAssigned counts the open conversations assigned to userID in the
// tenant on the context.
func (c *LoadCounter) CountOpenAssigned(ctx context.Context, userID string) (int, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, err
	}
	n, err := c.coll.CountDocuments(ctx, bson.M{
		"tenant_id":   tenantID,
		"assignee_id": userID,
		"status":      bson.M{"$in": openStatuses},
	})
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	return int(n), nil
}

var _ repository.LoadCounter = (*LoadCounter)(nil)
