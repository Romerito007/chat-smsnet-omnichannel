// Package attachments is the Mongo implementation of the attachment repository.
package attachments

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/attachments/repository"
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
	return &Repository{coll: db.Collection("attachments")}
}

func toDoc(a *entity.Attachment) models.AttachmentRecord {
	return models.AttachmentRecord{
		ID:              a.ID,
		TenantID:        a.TenantID,
		ConversationID:  a.ConversationID,
		MessageID:       a.MessageID,
		Filename:        a.Filename,
		ContentType:     a.ContentType,
		Size:            a.Size,
		StorageProvider: a.StorageProvider,
		StorageKey:      a.StorageKey,
		SignedURL:       a.SignedURL,
		Status:          string(a.Status),
		CreatedBy:       a.CreatedBy,
		CreatedAt:       a.CreatedAt,
	}
}

func toEntity(d *models.AttachmentRecord) *entity.Attachment {
	return &entity.Attachment{
		ID:              d.ID,
		TenantID:        d.TenantID,
		ConversationID:  d.ConversationID,
		MessageID:       d.MessageID,
		Filename:        d.Filename,
		ContentType:     d.ContentType,
		Size:            d.Size,
		StorageProvider: d.StorageProvider,
		StorageKey:      d.StorageKey,
		SignedURL:       d.SignedURL,
		Status:          entity.Status(d.Status),
		CreatedBy:       d.CreatedBy,
		CreatedAt:       d.CreatedAt,
	}
}

func (r *Repository) Create(ctx context.Context, a *entity.Attachment) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, toDoc(a))
	return mongodb.MapError(err)
}

func (r *Repository) Update(ctx context.Context, a *entity.Attachment) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": a.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"message_id": a.MessageID,
			"signed_url": a.SignedURL,
			"status":     string(a.Status),
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

func (r *Repository) FindByID(ctx context.Context, id string) (*entity.Attachment, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var d models.AttachmentRecord
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&d); err != nil {
		return nil, mongodb.MapError(err)
	}
	return toEntity(&d), nil
}

var _ repository.Repository = (*Repository)(nil)
