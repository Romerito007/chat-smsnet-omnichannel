// Package audit is the Mongo implementation of the audit-log repository. Logs
// are append-only; the audit.compact job and the privacy RetentionPolicy enforce
// retention.
package audit

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/audit/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/audit/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// Repository implements repository.Repository over MongoDB.
type Repository struct {
	coll *mongo.Collection
}

// New builds the repository.
func New(db *mongo.Database) *Repository {
	return &Repository{coll: db.Collection("audit_logs")}
}

func (r *Repository) Create(ctx context.Context, l *entity.AuditLog) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	doc := models.AuditLog{
		ID:           l.ID,
		TenantID:     tenantID,
		ActorID:      l.ActorID,
		ActorType:    l.ActorType,
		Action:       l.Action,
		ResourceType: l.ResourceType,
		ResourceID:   l.ResourceID,
		IP:           l.IP,
		UserAgent:    l.UserAgent,
		Data:         l.Data,
		CreatedAt:    l.CreatedAt,
	}
	_, err = r.coll.InsertOne(ctx, doc)
	return mongodb.MapError(err)
}

func (r *Repository) List(ctx context.Context, f repository.Filter, page shared.PageRequest) ([]*entity.AuditLog, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	base := bson.M{"tenant_id": tenantID}
	if f.Action != "" {
		base["action"] = bson.M{"$regex": "^" + f.Action}
	}
	if f.ResourceID != "" {
		base["resource_id"] = f.ResourceID
	}
	if f.ActorID != "" {
		base["actor_id"] = f.ActorID
	}
	filter := mongodb.ApplyKeyset(base, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.AuditLog
	for c.Next(ctx) {
		var d models.AuditLog
		if err := c.Decode(&d); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, &entity.AuditLog{
			ID:           d.ID,
			TenantID:     d.TenantID,
			ActorID:      d.ActorID,
			ActorType:    d.ActorType,
			Action:       d.Action,
			ResourceType: d.ResourceType,
			ResourceID:   d.ResourceID,
			IP:           d.IP,
			UserAgent:    d.UserAgent,
			Data:         d.Data,
			CreatedAt:    d.CreatedAt,
		})
	}
	return out, mongodb.MapError(c.Err())
}

var _ repository.Repository = (*Repository)(nil)
