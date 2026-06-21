// Package privacy is the Mongo implementation of the privacy domain Store. It
// reads across the contacts/conversations/messages/csat collections to assemble
// exports, hard-deletes a contact and all its satellite data for erasure (right
// to be forgotten), and applies the per-tenant RetentionPolicy — always
// tenant-scoped from context and always skipping data under an active legal hold.
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
	attachments   *mongo.Collection
	csat          *mongo.Collection
	sla           *mongo.Collection
	mcpApprovals  *mongo.Collection
	mcpCallLogs   *mongo.Collection
	copilotLogs   *mongo.Collection
	ruleEvalLogs  *mongo.Collection
	inbound       *mongo.Collection
	providerLogs  *mongo.Collection
	deals         *mongo.Collection
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
		attachments:   db.Collection("attachments"),
		csat:          db.Collection("csat_responses"),
		sla:           db.Collection("sla_trackings"),
		mcpApprovals:  db.Collection("mcp_approvals"),
		mcpCallLogs:   db.Collection("mcp_call_logs"),
		copilotLogs:   db.Collection("copilot_logs"),
		ruleEvalLogs:  db.Collection("rule_evaluation_logs"),
		inbound:       db.Collection("inbound_messages"),
		providerLogs:  db.Collection("provider_query_logs"),
		deals:         db.Collection("deals"),
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
			ID:         c.ID,
			Name:       c.Name,
			Phone:      c.Phone,
			Document:   c.Document,
			Anonymized: c.Anonymized,
			CreatedAt:  c.CreatedAt,
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

// --- Erasure (right to be forgotten) ---

// LinkedDeals lists deals tied to the contact directly (contact_id) or through
// any of the contact's conversations (conversation_ids).
func (r *Repository) LinkedDeals(ctx context.Context, contactID string) ([]privrepo.DealLink, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	convIDs, err := r.contactConversationIDs(ctx, tenantID, contactID)
	if err != nil {
		return nil, err
	}
	cur, err := r.deals.Find(ctx, dealLinkFilter(tenantID, contactID, convIDs))
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []privrepo.DealLink
	for cur.Next(ctx) {
		var d models.Deal
		if err := cur.Decode(&d); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, privrepo.DealLink{ID: d.ID, Title: d.Title})
	}
	return out, mongodb.MapError(cur.Err())
}

// EraseContact removes the contact and every document carrying its personal data
// or communications, returning the storage keys to purge. It is NOT transactional
// (Mongo cross-collection): keys are collected before the rows are deleted, and
// the order ends with the contact row as the commit point, so a retry after a
// partial failure re-derives the same targets and is idempotent.
func (r *Repository) EraseContact(ctx context.Context, contactID string, unlinkDeals bool) (privrepo.EraseResult, error) {
	var res privrepo.EraseResult
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return res, err
	}

	// 404 a missing contact up front (and grab the avatar blob to purge).
	var c models.Contact
	if err := r.contacts.FindOne(ctx, bson.M{"_id": contactID, "tenant_id": tenantID}).Decode(&c); err != nil {
		return res, mongodb.MapError(err)
	}

	convIDs, err := r.contactConversationIDs(ctx, tenantID, contactID)
	if err != nil {
		return res, err
	}

	// Sever (never delete) the linked deals so the company keeps its CRM record.
	dealsUnlinked := 0
	if unlinkDeals {
		n, err := r.unlinkDeals(ctx, tenantID, contactID, convIDs)
		if err != nil {
			return res, err
		}
		dealsUnlinked = n
	}

	// Cascade the conversation-scoped satellites + media blobs (shared with
	// retention so neither path strands a deleted conversation's data/media).
	res, err = r.cascadeConversationData(ctx, tenantID, convIDs)
	if err != nil {
		return res, err
	}
	res.DealsUnlinked = dealsUnlinked

	// The avatar attachment is not conversation-scoped, so purge it separately.
	if c.AvatarAttachmentID != "" {
		avatarKeys, err := r.attachmentBlobKeys(ctx, tenantID, nil, c.AvatarAttachmentID)
		if err != nil {
			return res, err
		}
		res.BlobKeys = append(res.BlobKeys, avatarKeys...)
	}
	res.ExportKeys, err = r.exportStorageKeys(ctx, tenantID, contactID)
	if err != nil {
		return res, err
	}

	// Contact-scoped catch-alls (cover docs not tied to a conversation).
	byContact := bson.M{"tenant_id": tenantID, "contact_id": contactID}
	if n, err := r.deleteCount(ctx, r.csat, byContact); err != nil {
		return res, err
	} else {
		res.CSAT = n
	}
	if n, err := r.deleteCount(ctx, r.providerLogs, byContact); err != nil {
		return res, err
	} else {
		res.ProviderQueryLogs += n
	}
	if n, err := r.deleteCount(ctx, r.exports, byContact); err != nil {
		return res, err
	} else {
		res.Exports = n
	}

	// Conversations, then the contact row (commit point).
	if n, err := r.deleteCount(ctx, r.conversations, bson.M{"tenant_id": tenantID, "contact_id": contactID}); err != nil {
		return res, err
	} else {
		res.Conversations = n
	}
	if _, err := r.contacts.DeleteOne(ctx, bson.M{"_id": contactID, "tenant_id": tenantID}); err != nil {
		return res, mongodb.MapError(err)
	}
	return res, nil
}

// cascadeConversationData deletes every conversation-scoped satellite document
// for convIDs and returns the per-collection delete counts plus the attachment
// media keys to purge (the attachment ROWS are deleted here; their physical
// blobs are purged by the caller). Shared by EraseContact and ApplyRetention so
// retention never strands a deleted conversation's satellites or media.
func (r *Repository) cascadeConversationData(ctx context.Context, tenantID string, convIDs []string) (privrepo.EraseResult, error) {
	var res privrepo.EraseResult
	if len(convIDs) == 0 {
		return res, nil
	}
	// Collect blob keys BEFORE deleting the attachment rows that reference them.
	keys, err := r.attachmentBlobKeys(ctx, tenantID, convIDs, "")
	if err != nil {
		return res, err
	}
	res.BlobKeys = keys
	byConv := bson.M{"tenant_id": tenantID, "conversation_id": bson.M{"$in": convIDs}}
	for _, d := range []struct {
		coll *mongo.Collection
		n    *int
	}{
		{r.messages, &res.Messages},
		{r.events, &res.Events},
		{r.attachments, &res.Attachments},
		{r.sla, &res.SLATrackings},
		{r.mcpApprovals, &res.MCPApprovals},
		{r.mcpCallLogs, &res.MCPCallLogs},
		{r.copilotLogs, &res.CopilotLogs},
		{r.ruleEvalLogs, &res.RuleEvalLogs},
		{r.inbound, &res.InboundMessages},
		{r.providerLogs, &res.ProviderQueryLogs},
	} {
		n, err := r.deleteCount(ctx, d.coll, byConv)
		if err != nil {
			return res, err
		}
		*d.n = n
	}
	return res, nil
}

// contactConversationIDs returns the ids of the contact's conversations.
func (r *Repository) contactConversationIDs(ctx context.Context, tenantID, contactID string) ([]string, error) {
	vals, err := r.conversations.Distinct(ctx, "_id", bson.M{"tenant_id": tenantID, "contact_id": contactID})
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out, nil
}

// attachmentBlobKeys gathers the storage keys of the conversations' attachments
// plus the contact's avatar attachment. The avatar attachment row (not tied to a
// conversation) is deleted here, since the conversation-scoped sweep won't catch
// it.
func (r *Repository) attachmentBlobKeys(ctx context.Context, tenantID string, convIDs []string, avatarID string) ([]string, error) {
	or := bson.A{}
	if len(convIDs) > 0 {
		or = append(or, bson.M{"conversation_id": bson.M{"$in": convIDs}})
	}
	if avatarID != "" {
		or = append(or, bson.M{"_id": avatarID})
	}
	if len(or) == 0 {
		return nil, nil
	}
	out, err := r.attachmentKeysFor(ctx, bson.M{"tenant_id": tenantID, "$or": or})
	if err != nil {
		return nil, err
	}
	if avatarID != "" {
		if _, err := r.attachments.DeleteOne(ctx, bson.M{"_id": avatarID, "tenant_id": tenantID}); err != nil {
			return nil, mongodb.MapError(err)
		}
	}
	return out, nil
}

// attachmentKeysFor returns the storage keys of the attachments matching filter
// (read-only; the rows are deleted separately by the caller).
func (r *Repository) attachmentKeysFor(ctx context.Context, filter bson.M) ([]string, error) {
	cur, err := r.attachments.Find(ctx, filter, options.Find().SetProjection(bson.M{"storage_key": 1}))
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []string
	for cur.Next(ctx) {
		var a models.AttachmentRecord
		if err := cur.Decode(&a); err != nil {
			return nil, mongodb.MapError(err)
		}
		if a.StorageKey != "" {
			out = append(out, a.StorageKey)
		}
	}
	return out, mongodb.MapError(cur.Err())
}

// exportStorageKeys gathers the object keys of the contact's export bundles.
func (r *Repository) exportStorageKeys(ctx context.Context, tenantID, contactID string) ([]string, error) {
	vals, err := r.exports.Distinct(ctx, "storage_key", bson.M{"tenant_id": tenantID, "contact_id": contactID})
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

// unlinkDeals severs the contact from its deals without deleting them: it clears
// contact_id on deals owned by the contact and pulls the contact's conversation
// ids from every deal's conversation_ids. The title and the rest of the deal are
// kept for the company to review.
func (r *Repository) unlinkDeals(ctx context.Context, tenantID, contactID string, convIDs []string) (int, error) {
	matched := map[string]struct{}{}
	ids, err := r.deals.Distinct(ctx, "_id", dealLinkFilter(tenantID, contactID, convIDs))
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	for _, v := range ids {
		if s, ok := v.(string); ok {
			matched[s] = struct{}{}
		}
	}
	if _, err := r.deals.UpdateMany(ctx,
		bson.M{"tenant_id": tenantID, "contact_id": contactID},
		bson.M{"$set": bson.M{"contact_id": "", "updated_at": time.Now().UTC()}},
	); err != nil {
		return 0, mongodb.MapError(err)
	}
	if len(convIDs) > 0 {
		if _, err := r.deals.UpdateMany(ctx,
			bson.M{"tenant_id": tenantID, "conversation_ids": bson.M{"$in": convIDs}},
			bson.M{
				"$pull": bson.M{"conversation_ids": bson.M{"$in": convIDs}},
				"$set":  bson.M{"updated_at": time.Now().UTC()},
			},
		); err != nil {
			return 0, mongodb.MapError(err)
		}
	}
	return len(matched), nil
}

// dealLinkFilter matches deals linked to the contact directly or via any of the
// contact's conversations.
func dealLinkFilter(tenantID, contactID string, convIDs []string) bson.M {
	or := bson.A{bson.M{"contact_id": contactID}}
	if len(convIDs) > 0 {
		or = append(or, bson.M{"conversation_ids": bson.M{"$in": convIDs}})
	}
	return bson.M{"tenant_id": tenantID, "$or": or}
}

// distinctIDs returns the distinct string values of field in coll matching filter.
func (r *Repository) distinctIDs(ctx context.Context, coll *mongo.Collection, field string, filter bson.M) ([]string, error) {
	vals, err := coll.Distinct(ctx, field, filter)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out, nil
}

// deleteCount deletes matching docs and returns the count removed.
func (r *Repository) deleteCount(ctx context.Context, coll *mongo.Collection, filter bson.M) (int, error) {
	res, err := coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, mongodb.MapError(err)
	}
	return int(res.DeletedCount), nil
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
		msgCutoff := cutoff(now, p.MessagesDays)
		// Purge the attachments (rows + media blobs) of the messages being retired:
		// an attachment is created with its message, so the same age cutoff applies.
		// This keeps message retention from stranding orphan media. Held
		// conversations are exempt.
		attFilter := bson.M{"tenant_id": tenantID, "created_at": bson.M{"$lte": msgCutoff}}
		if len(heldConvIDs) > 0 {
			attFilter["conversation_id"] = bson.M{"$nin": heldConvIDs}
		}
		keys, err := r.attachmentKeysFor(ctx, attFilter)
		if err != nil {
			return res, err
		}
		res.BlobKeys = append(res.BlobKeys, keys...)
		if na, err := r.deleteCount(ctx, r.attachments, attFilter); err != nil {
			return res, err
		} else {
			res.SatelliteDocs += na
		}

		n, err := r.deleteMany(ctx, r.messages, bson.M{
			"tenant_id":  tenantID,
			"created_at": bson.M{"$lte": msgCutoff},
		}, "conversation_id", heldConvIDs)
		if err != nil {
			return res, err
		}
		res.Messages = n
	}

	if p.ClosedConversationsDays > 0 {
		// Resolve the exact conversation ids being retired so we can cascade their
		// satellites + media — deleting the conversation row alone would strand
		// messages, attachments (and their blobs), CSAT, SLA, MCP, etc.
		convFilter := bson.M{
			"tenant_id": tenantID,
			"status":    bson.M{"$in": closedStatuses},
			"closed_at": bson.M{"$lte": cutoff(now, p.ClosedConversationsDays)},
		}
		if len(heldConvIDs) > 0 {
			convFilter["_id"] = bson.M{"$nin": heldConvIDs}
		}
		convIDs, err := r.distinctIDs(ctx, r.conversations, "_id", convFilter)
		if err != nil {
			return res, err
		}
		casc, err := r.cascadeConversationData(ctx, tenantID, convIDs)
		if err != nil {
			return res, err
		}
		res.SatelliteDocs += casc.Documents()
		res.BlobKeys = append(res.BlobKeys, casc.BlobKeys...)
		n, err := r.deleteCount(ctx, r.conversations, convFilter)
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
