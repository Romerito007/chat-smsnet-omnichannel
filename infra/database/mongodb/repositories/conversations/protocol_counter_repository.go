package conversations

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/conversations/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
)

// ProtocolCounterRepository hands out per-(tenant, year) protocol sequences using
// a single atomic findAndModify $inc upsert, so two conversations opening at the
// same instant never get the same number.
type ProtocolCounterRepository struct {
	coll *mongo.Collection
}

// NewProtocolCounterRepository builds the repository.
func NewProtocolCounterRepository(db *mongo.Database) *ProtocolCounterRepository {
	return &ProtocolCounterRepository{coll: db.Collection("protocol_counters")}
}

// NextSequence atomically increments and returns the next sequence for the tenant
// + year. The document id is "<tenantID>:<year>"; $inc on a missing document
// upserts it starting at 1.
func (r *ProtocolCounterRepository) NextSequence(ctx context.Context, tenantID string, year int) (int64, error) {
	id := fmt.Sprintf("%s:%d", tenantID, year)
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)
	var doc struct {
		Seq int64 `bson:"seq"`
	}
	err := r.coll.FindOneAndUpdate(ctx,
		bson.M{"_id": id},
		bson.M{
			"$inc":         bson.M{"seq": 1},
			"$setOnInsert": bson.M{"tenant_id": tenantID, "year": year},
		},
		opts,
	).Decode(&doc)
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	return doc.Seq, nil
}

var _ repository.ProtocolCounterRepository = (*ProtocolCounterRepository)(nil)
