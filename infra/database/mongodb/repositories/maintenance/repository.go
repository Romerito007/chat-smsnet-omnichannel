// Package maintenance is the Mongo implementation of the maintenance repository
// (audit retention + reports snapshots).
package maintenance

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/maintenance"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
)

// Repository implements maintenance.Repository.
type Repository struct {
	auditLogs     *mongo.Collection
	conversations *mongo.Collection
	messages      *mongo.Collection
	snapshots     *mongo.Collection
}

// New builds the repository.
func New(db *mongo.Database) *Repository {
	return &Repository{
		auditLogs:     db.Collection("audit_logs"),
		conversations: db.Collection("conversations"),
		messages:      db.Collection("messages"),
		snapshots:     db.Collection("reports_snapshots"),
	}
}

func (r *Repository) DeleteAuditBefore(ctx context.Context, before time.Time) (int, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, err
	}
	res, err := r.auditLogs.DeleteMany(ctx, bson.M{"tenant_id": tenantID, "created_at": bson.M{"$lte": before}})
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	return int(res.DeletedCount), nil
}

func (r *Repository) DayCounts(ctx context.Context, from, to time.Time) (maintenance.Counts, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return maintenance.Counts{}, err
	}
	closedStatuses := bson.A{"closed", "resolved", "archived"}

	open, err := r.conversations.CountDocuments(ctx, bson.M{"tenant_id": tenantID, "status": bson.M{"$nin": closedStatuses}})
	if err != nil {
		return maintenance.Counts{}, mongodb.MapError(err)
	}
	closed, err := r.conversations.CountDocuments(ctx, bson.M{
		"tenant_id": tenantID, "status": bson.M{"$in": closedStatuses},
		"closed_at": bson.M{"$gte": from, "$lt": to},
	})
	if err != nil {
		return maintenance.Counts{}, mongodb.MapError(err)
	}
	msgs, err := r.messages.CountDocuments(ctx, bson.M{
		"tenant_id":  tenantID,
		"created_at": bson.M{"$gte": from, "$lt": to},
	})
	if err != nil {
		return maintenance.Counts{}, mongodb.MapError(err)
	}
	return maintenance.Counts{
		OpenConversations:   int(open),
		ClosedConversations: int(closed),
		Messages:            int(msgs),
	}, nil
}

func (r *Repository) UpsertSnapshot(ctx context.Context, s maintenance.Snapshot) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	id := s.TenantID + ":" + s.Date
	_, err := r.snapshots.UpdateOne(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"tenant_id":            s.TenantID,
			"date":                 s.Date,
			"open_conversations":   s.OpenConversations,
			"closed_conversations": s.ClosedConversations,
			"messages":             s.Messages,
			"generated_at":         s.GeneratedAt,
		}},
		options.Update().SetUpsert(true),
	)
	return mongodb.MapError(err)
}

var _ maintenance.Repository = (*Repository)(nil)
