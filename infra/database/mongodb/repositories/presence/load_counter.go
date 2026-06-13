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
		"assigned_to": userID,
		"status":      bson.M{"$in": openStatuses},
	})
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	return int(n), nil
}

// OpenAssignedLoads aggregates the open, assigned conversations of the tenant by
// assignee in a single query: $match {tenant, status open, assigned} → $group
// {_id:$assigned_to, load:$sum 1}.
func (c *LoadCounter) OpenAssignedLoads(ctx context.Context) (map[string]int, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"tenant_id":   tenantID,
			"status":      bson.M{"$in": openStatuses},
			"assigned_to": bson.M{"$nin": bson.A{"", nil}},
		}}},
		{{Key: "$group", Value: bson.M{"_id": "$assigned_to", "load": bson.M{"$sum": 1}}}},
	}
	cur, err := c.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	out := map[string]int{}
	for cur.Next(ctx) {
		var row struct {
			UserID string `bson:"_id"`
			Load   int    `bson:"load"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, mongodb.MapError(err)
		}
		out[row.UserID] = row.Load
	}
	return out, mongodb.MapError(cur.Err())
}

var _ repository.LoadCounter = (*LoadCounter)(nil)
