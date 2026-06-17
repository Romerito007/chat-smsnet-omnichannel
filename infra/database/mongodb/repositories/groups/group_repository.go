// Package groups is the Mongo implementation of the groups repository. Every
// operation is scoped by the tenant from the context.
package groups

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/groups/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/groups/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/groups/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// Repository implements repository.GroupRepository over MongoDB.
type Repository struct {
	coll  *mongo.Collection
	clock shared.Clock
}

// New builds the repository.
func New(db *mongo.Database, clock shared.Clock) *Repository {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Repository{coll: db.Collection("whatsapp_groups"), clock: clock}
}

func (r *Repository) UpsertBatch(ctx context.Context, channelID string, groups []contracts.UpsertGroup) (int, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return 0, err
	}
	now := r.clock.Now()
	ops := make([]mongo.WriteModel, 0, len(groups))
	for _, g := range groups {
		set := bson.M{
			"channel_id":   channelID,
			"name":         g.Name,
			"description":  g.Description,
			"participants": g.Participants,
			"group_admins": g.GroupAdmins,
			"company_id":   g.CompanyID,
			"whatsapp_wid": g.WhatsAppWID,
			"owner_name":   g.OwnerName,
			"owner_jid":    g.OwnerJID,
			"activated":    g.Activated,
			"synced_at":    now,
			"updated_at":   now,
		}
		// tenant_id + group_jid come from the filter on insert; attend defaults to
		// true ONLY on insert, so a re-sync never resets the operator's choice.
		setOnInsert := bson.M{"_id": shared.NewID(), "attend": true, "created_at": now}
		ops = append(ops, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"tenant_id": tenantID, "group_jid": g.GroupJID}).
			SetUpdate(bson.M{"$set": set, "$setOnInsert": setOnInsert}).
			SetUpsert(true))
	}
	res, err := r.coll.BulkWrite(ctx, ops, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	return int(res.UpsertedCount + res.ModifiedCount), nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (*entity.Group, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.WhatsAppGroup
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *Repository) FindByJID(ctx context.Context, groupJID string) (*entity.Group, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.WhatsAppGroup
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "group_jid": groupJID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func (r *Repository) List(ctx context.Context, f contracts.ListFilter, page shared.PageRequest) ([]*entity.Group, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	base := bson.M{"tenant_id": tenantID}
	if f.ChannelID != "" {
		base["channel_id"] = f.ChannelID
	}
	if f.Attend != nil {
		base["attend"] = *f.Attend
	}
	if f.Q != "" {
		// Text index on {name, description}; matching is fast, pagination stays keyset.
		base["$text"] = bson.M{"$search": f.Q}
	}
	filter := mongodb.ApplyKeyset(base, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.Group
	for c.Next(ctx) {
		var m models.WhatsAppGroup
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, toEntity(&m))
	}
	return out, mongodb.MapError(c.Err())
}

func (r *Repository) SetAttend(ctx context.Context, id string, attend bool) (*entity.Group, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.WhatsAppGroup
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	err = r.coll.FindOneAndUpdate(ctx,
		bson.M{"_id": id, "tenant_id": tenantID},
		bson.M{"$set": bson.M{"attend": attend, "updated_at": r.clock.Now()}},
		opts,
	).Decode(&m)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&m), nil
}

func toEntity(m *models.WhatsAppGroup) *entity.Group {
	return &entity.Group{
		ID:           m.ID,
		TenantID:     m.TenantID,
		ChannelID:    m.ChannelID,
		GroupJID:     m.GroupJID,
		Name:         m.Name,
		Description:  m.Description,
		Participants: m.Participants,
		GroupAdmins:  m.GroupAdmins,
		CompanyID:    m.CompanyID,
		WhatsAppWID:  m.WhatsAppWID,
		OwnerName:    m.OwnerName,
		OwnerJID:     m.OwnerJID,
		Activated:    m.Activated,
		Attend:       m.Attend,
		SyncedAt:     m.SyncedAt,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

var _ repository.GroupRepository = (*Repository)(nil)
