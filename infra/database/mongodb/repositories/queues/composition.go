package queues

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/queues/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
)

// CompositionCounter counts queue composition over the conversations collection.
// It implements contracts.CompositionCounter and is tenant-scoped from context.
type CompositionCounter struct {
	conversations *mongo.Collection
}

// NewCompositionCounter builds the counter.
func NewCompositionCounter(db *mongo.Database) *CompositionCounter {
	return &CompositionCounter{conversations: db.Collection("conversations")}
}

// QueueComposition returns the number of conversations still waiting in the queue
// (status=queued, queue_id=queueID) and the number currently assigned within the
// queue's sector (status=assigned, sector_id=sectorID — queue_id is cleared on
// assignment).
func (c *CompositionCounter) QueueComposition(ctx context.Context, sectorID, queueID string) (int, int, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, 0, err
	}
	waiting, err := c.conversations.CountDocuments(ctx, bson.M{
		"tenant_id": tenantID,
		"queue_id":  queueID,
		"status":    "queued",
	})
	if err != nil {
		return 0, 0, mongodb.MapError(err)
	}
	assignedFilter := bson.M{
		"tenant_id": tenantID,
		"status":    "assigned",
	}
	if sectorID != "" {
		assignedFilter["sector_id"] = sectorID
	}
	assigned, err := c.conversations.CountDocuments(ctx, assignedFilter)
	if err != nil {
		return 0, 0, mongodb.MapError(err)
	}
	return int(waiting), int(assigned), nil
}

var _ contracts.CompositionCounter = (*CompositionCounter)(nil)
