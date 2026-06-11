// Package privacy is the Mongo implementation of the privacy domain Store. It
// reads across the contacts/conversations/messages/csat collections to assemble
// exports, overwrites PII for anonymization, and applies the per-tenant
// RetentionPolicy — always tenant-scoped from context and always skipping data
// under an active legal hold.
package privacy

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/entity"
	privrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/privacy/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
)

// closedStatuses are the terminal conversation states subject to the
// closed-conversations retention.
var closedStatuses = bson.A{"closed", "resolved", "archived"}

// Repository implements privrepo.Store over MongoDB.
type Repository struct {
	contacts      *mongo.Collection
	conversations *mongo.Collection
	messages      *mongo.Collection
	events        *mongo.Collection
	csat          *mongo.Collection
	notifications *mongo.Collection
	auditLogs     *mongo.Collection
	retention     *mongo.Collection
	exports       *mongo.Collection
	legalHolds    *mongo.Collection
}

// New builds the repository.
func New(db *mongo.Database) *Repository {
	return &Repository{
		contacts:      db.Collection("contacts"),
		conversations: db.Collection("conversations"),
		messages:      db.Collection("messages"),
		events:        db.Collection("conversation_events"),
		csat:          db.Collection("csat_responses"),
		notifications: db.Collection("notifications"),
		auditLogs:     db.Collection("audit_logs"),
		retention:     db.Collection("retention_policies"),
		exports:       db.Collection("privacy_exports"),
		legalHolds:    db.Collection("legal_holds"),
	}
}

// --- Retention policy ---

func (r *Repository) GetRetention(ctx context.Context) (*entity.RetentionPolicy, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var d models.RetentionPolicy
	err = r.retention.FindOne(ctx, bson.M{"_id": tenantID}).Decode(&d)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	return &entity.RetentionPolicy{
		TenantID:                d.TenantID,
		MessagesDays:            d.MessagesDays,
		ClosedConversationsDays: d.ClosedConversationsDays,
		TechnicalLogsDays:       d.TechnicalLogsDays,
		AuditLogsDays:           d.AuditLogsDays,
		NotificationsDays:       d.NotificationsDays,
		UpdatedAt:               d.UpdatedAt,
	}, nil
}

func (r *Repository) SaveRetention(ctx context.Context, p *entity.RetentionPolicy) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	_, err = r.retention.UpdateOne(ctx,
		bson.M{"_id": tenantID},
		bson.M{"$set": bson.M{
			"tenant_id":                 tenantID,
			"messages_days":             p.MessagesDays,
			"closed_conversations_days": p.ClosedConversationsDays,
			"technical_logs_days":       p.TechnicalLogsDays,
			"audit_logs_days":           p.AuditLogsDays,
			"notifications_days":        p.NotificationsDays,
			"updated_at":                p.UpdatedAt,
		}},
		options.Update().SetUpsert(true),
	)
	return mongodb.MapError(err)
}

// --- Export requests ---

func (r *Repository) CreateExport(ctx context.Context, e *entity.ExportRequest) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	e.TenantID = tenantID
	_, err = r.exports.InsertOne(ctx, models.PrivacyExport{
		ID:          e.ID,
		TenantID:    tenantID,
		ContactID:   e.ContactID,
		Status:      string(e.Status),
		RequestedBy: e.RequestedBy,
		CreatedAt:   e.CreatedAt,
	})
	return mongodb.MapError(err)
}

func (r *Repository) UpdateExport(ctx context.Context, e *entity.ExportRequest) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.exports.UpdateOne(ctx,
		bson.M{"_id": e.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"status":       string(e.Status),
			"storage_key":  e.StorageKey,
			"download_url": e.DownloadURL,
			"expires_at":   e.ExpiresAt,
			"error":        e.Error,
			"completed_at": e.CompletedAt,
		}},
	)
	if err != nil {
		return mongodb.MapError(err)
	}
	if res.MatchedCount == 0 {
		return mongodb.MapError(mongo.ErrNoDocuments)
	}
	return nil
}

func (r *Repository) FindExport(ctx context.Context, id string) (*entity.ExportRequest, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var d models.PrivacyExport
	if err := r.exports.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&d); err != nil {
		return nil, mongodb.MapError(err)
	}
	return &entity.ExportRequest{
		ID:          d.ID,
		TenantID:    d.TenantID,
		ContactID:   d.ContactID,
		Status:      entity.ExportStatus(d.Status),
		RequestedBy: d.RequestedBy,
		StorageKey:  d.StorageKey,
		DownloadURL: d.DownloadURL,
		ExpiresAt:   d.ExpiresAt,
		Error:       d.Error,
		CreatedAt:   d.CreatedAt,
		CompletedAt: d.CompletedAt,
	}, nil
}

// --- Export bundle assembly ---

func (r *Repository) CollectBundle(ctx context.Context, contactID string) (*privrepo.ExportBundle, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var c models.PrivacyContact
	if err := r.contacts.FindOne(ctx, bson.M{"_id": contactID, "tenant_id": tenantID}).Decode(&c); err != nil {
		return nil, mongodb.MapError(err)
	}
	bundle := &privrepo.ExportBundle{
		Contact: privrepo.ContactData{
			ID:        c.ID,
			Name:      c.Name,
			Phone:     c.Phone,
			Document:  c.Document,
			CreatedAt: c.CreatedAt,
		},
	}
	for _, id := range c.Identities {
		bundle.Contact.Identities = append(bundle.Contact.Identities, privrepo.IdentityData{
			Channel: id.Channel, ExternalID: id.ExternalID,
		})
	}

	// Conversations (oldest first) + their messages.
	convCur, err := r.conversations.Find(ctx,
		bson.M{"tenant_id": tenantID, "contact_id": contactID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}),
	)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = convCur.Close(ctx) }()
	for convCur.Next(ctx) {
		var cv models.PrivacyConversation
		if err := convCur.Decode(&cv); err != nil {
			return nil, mongodb.MapError(err)
		}
		cd := privrepo.ConversationData{
			ID:        cv.ID,
			Channel:   cv.Channel,
			Status:    cv.Status,
			CreatedAt: cv.CreatedAt,
			ClosedAt:  cv.ClosedAt,
		}
		msgs, err := r.messagesFor(ctx, tenantID, cv.ID)
		if err != nil {
			return nil, err
		}
		cd.Messages = msgs
		bundle.Conversations = append(bundle.Conversations, cd)
	}
	if err := convCur.Err(); err != nil {
		return nil, mongodb.MapError(err)
	}

	// CSAT responses for the contact.
	csatCur, err := r.csat.Find(ctx,
		bson.M{"tenant_id": tenantID, "contact_id": contactID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}),
	)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = csatCur.Close(ctx) }()
	for csatCur.Next(ctx) {
		var cs models.PrivacyCSAT
		if err := csatCur.Decode(&cs); err != nil {
			return nil, mongodb.MapError(err)
		}
		bundle.CSAT = append(bundle.CSAT, privrepo.CSATData{
			ID:             cs.ID,
			ConversationID: cs.ConversationID,
			Score:          cs.Score,
			Comment:        cs.Comment,
			Status:         cs.Status,
			CreatedAt:      cs.CreatedAt,
		})
	}
	return bundle, mongodb.MapError(csatCur.Err())
}

func (r *Repository) messagesFor(ctx context.Context, tenantID, convID string) ([]privrepo.MessageData, error) {
	cur, err := r.messages.Find(ctx,
		bson.M{"tenant_id": tenantID, "conversation_id": convID, "deleted_at": bson.M{"$exists": false}},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}),
	)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []privrepo.MessageData
	for cur.Next(ctx) {
		var m models.PrivacyMessage
		if err := cur.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, privrepo.MessageData{
			ID:         m.ID,
			Direction:  m.Direction,
			SenderType: m.SenderType,
			Type:       m.MessageType,
			Text:       m.Text,
			CreatedAt:  m.CreatedAt,
		})
	}
	return out, mongodb.MapError(cur.Err())
}

// --- Anonymization ---

func (r *Repository) AnonymizeContact(ctx context.Context, contactID string, a privrepo.Anonymized) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.contacts.UpdateOne(ctx,
		bson.M{"_id": contactID, "tenant_id": tenantID},
		bson.M{
			"$set": bson.M{
				"name":       a.Name,
				"phone":      a.Phone,
				"document":   a.Document,
				"anonymized": true,
				// Clear channel-identity handles (PII) while keeping the contact row
				// and id so conversations/metrics stay linked (integrity).
				"identities.$[].external_id": "",
				"updated_at":                 time.Now().UTC(),
			},
		},
	)
	if err != nil {
		return mongodb.MapError(err)
	}
	if res.MatchedCount == 0 {
		return mongodb.MapError(mongo.ErrNoDocuments)
	}
	return nil
}

func (r *Repository) UpdateMessageText(ctx context.Context, id, text string) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	_, err = r.messages.UpdateOne(ctx,
		bson.M{"_id": id, "tenant_id": tenantID},
		bson.M{"$set": bson.M{"text": text}},
	)
	return mongodb.MapError(err)
}

// --- Legal hold ---

func (r *Repository) HasActiveLegalHold(ctx context.Context, contactID string, at time.Time) (bool, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return false, err
	}
	n, err := r.legalHolds.CountDocuments(ctx, activeHoldFilter(tenantID, at, contactID))
	if err != nil {
		return false, mongodb.MapError(err)
	}
	return n > 0, nil
}

// activeHoldFilter matches legal holds in force at `at`: an indefinite hold
// (no/zero until) or one whose deadline is still in the future. When contactID is
// empty it matches across the tenant.
func activeHoldFilter(tenantID string, at time.Time, contactID string) bson.M {
	f := bson.M{
		"tenant_id": tenantID,
		"$or": bson.A{
			bson.M{"until": bson.M{"$exists": false}},
			bson.M{"until": time.Time{}},
			bson.M{"until": bson.M{"$gt": at}},
		},
	}
	if contactID != "" {
		f["contact_id"] = contactID
	}
	return f
}

// --- Retention application ---

func (r *Repository) ApplyRetention(ctx context.Context, p entity.RetentionPolicy, now time.Time) (privrepo.RetentionResult, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return privrepo.RetentionResult{}, err
	}

	// Conversations belonging to contacts under an active legal hold are exempt
	// (along with their messages/events).
	heldConvIDs, err := r.heldConversationIDs(ctx, tenantID, now)
	if err != nil {
		return privrepo.RetentionResult{}, err
	}

	var res privrepo.RetentionResult

	if p.MessagesDays > 0 {
		n, err := r.deleteMany(ctx, r.messages, bson.M{
			"tenant_id":  tenantID,
			"created_at": bson.M{"$lte": cutoff(now, p.MessagesDays)},
		}, "conversation_id", heldConvIDs)
		if err != nil {
			return res, err
		}
		res.Messages = n
	}

	if p.ClosedConversationsDays > 0 {
		n, err := r.deleteMany(ctx, r.conversations, bson.M{
			"tenant_id": tenantID,
			"status":    bson.M{"$in": closedStatuses},
			"closed_at": bson.M{"$lte": cutoff(now, p.ClosedConversationsDays)},
		}, "_id", heldConvIDs)
		if err != nil {
			return res, err
		}
		res.ClosedConversations = n
	}

	if p.TechnicalLogsDays > 0 {
		n, err := r.deleteMany(ctx, r.events, bson.M{
			"tenant_id":  tenantID,
			"created_at": bson.M{"$lte": cutoff(now, p.TechnicalLogsDays)},
		}, "conversation_id", heldConvIDs)
		if err != nil {
			return res, err
		}
		res.TechnicalLogs = n
	}

	if p.AuditLogsDays > 0 {
		n, err := r.deleteMany(ctx, r.auditLogs, bson.M{
			"tenant_id":  tenantID,
			"created_at": bson.M{"$lte": cutoff(now, p.AuditLogsDays)},
		}, "", nil)
		if err != nil {
			return res, err
		}
		res.AuditLogs = n
	}

	if p.NotificationsDays > 0 {
		n, err := r.deleteMany(ctx, r.notifications, bson.M{
			"tenant_id":  tenantID,
			"created_at": bson.M{"$lte": cutoff(now, p.NotificationsDays)},
		}, "", nil)
		if err != nil {
			return res, err
		}
		res.Notifications = n
	}

	return res, nil
}

// heldConversationIDs returns the ids of conversations whose contact is under an
// active legal hold.
func (r *Repository) heldConversationIDs(ctx context.Context, tenantID string, now time.Time) ([]string, error) {
	heldContacts, err := r.legalHolds.Distinct(ctx, "contact_id", activeHoldFilter(tenantID, now, ""))
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	if len(heldContacts) == 0 {
		return nil, nil
	}
	convIDs, err := r.conversations.Distinct(ctx, "_id", bson.M{
		"tenant_id":  tenantID,
		"contact_id": bson.M{"$in": heldContacts},
	})
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	out := make([]string, 0, len(convIDs))
	for _, v := range convIDs {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out, nil
}

// deleteMany deletes matching docs, optionally excluding those whose excludeField
// is in the exclude set (legal-hold exemption).
func (r *Repository) deleteMany(ctx context.Context, coll *mongo.Collection, filter bson.M, excludeField string, exclude []string) (int, error) {
	if excludeField != "" && len(exclude) > 0 {
		filter[excludeField] = bson.M{"$nin": exclude}
	}
	res, err := coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	return int(res.DeletedCount), nil
}

func cutoff(now time.Time, days int) time.Time {
	return now.AddDate(0, 0, -days)
}

var _ privrepo.Store = (*Repository)(nil)
