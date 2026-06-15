package providerhub

import (
	"context"
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/secrets"
)

// ProfileRepository implements repository.ProfileRepository over the
// isp_profiles collection. The ISP credentials are encrypted at rest.
type ProfileRepository struct {
	coll   *mongo.Collection
	cipher *secrets.Cipher
}

// NewProfileRepository builds the repository.
func NewProfileRepository(db *mongo.Database, cipher *secrets.Cipher) *ProfileRepository {
	return &ProfileRepository{coll: db.Collection("isp_profiles"), cipher: cipher}
}

func (r *ProfileRepository) Create(ctx context.Context, p *entity.ISPProfile) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	m, err := r.toModel(p)
	if err != nil {
		return err
	}
	_, err = r.coll.InsertOne(ctx, m)
	return mongodb.MapError(err)
}

func (r *ProfileRepository) Update(ctx context.Context, p *entity.ISPProfile) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	m, err := r.toModel(p)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": p.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"label":                 m.Label,
			"isp_type":              m.ISPType,
			"encrypted_credentials": m.EncryptedCredentials,
			"transports":            m.Transports,
			"is_default":            m.IsDefault,
			"options":               m.Options,
			"timeout_ms":            m.TimeoutMs,
			"enabled":               m.Enabled,
			"updated_at":            p.UpdatedAt,
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

func (r *ProfileRepository) Delete(ctx context.Context, id string) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id, "tenant_id": tenantID})
	if err != nil {
		return mongodb.MapError(err)
	}
	if res.DeletedCount == 0 {
		return mongodb.MapError(mongo.ErrNoDocuments)
	}
	return nil
}

func (r *ProfileRepository) FindByID(ctx context.Context, id string) (*entity.ISPProfile, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.ISPProfile
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return r.toEntity(&m)
}

func (r *ProfileRepository) FindDefault(ctx context.Context) (*entity.ISPProfile, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.ISPProfile
	if err := r.coll.FindOne(ctx, bson.M{"tenant_id": tenantID, "is_default": true}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return r.toEntity(&m)
}

func (r *ProfileRepository) List(ctx context.Context) ([]*entity.ISPProfile, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := r.coll.Find(ctx, bson.M{"tenant_id": tenantID})
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []*entity.ISPProfile
	for cur.Next(ctx) {
		var m models.ISPProfile
		if err := cur.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		e, err := r.toEntity(&m)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, mongodb.MapError(cur.Err())
}

func (r *ProfileRepository) ClearDefault(ctx context.Context) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	_, err = r.coll.UpdateMany(ctx,
		bson.M{"tenant_id": tenantID, "is_default": true},
		bson.M{"$set": bson.M{"is_default": false}},
	)
	return mongodb.MapError(err)
}

func (r *ProfileRepository) toModel(p *entity.ISPProfile) (models.ISPProfile, error) {
	encCreds := ""
	if len(p.Credentials) > 0 {
		raw, err := json.Marshal(p.Credentials)
		if err != nil {
			return models.ISPProfile{}, apperror.Internal("marshal credentials").Wrap(err)
		}
		v, err := r.cipher.Encrypt(string(raw))
		if err != nil {
			return models.ISPProfile{}, apperror.Internal("encrypt credentials").Wrap(err)
		}
		encCreds = v
	}
	m := models.ISPProfile{
		Label:                p.Label,
		ISPType:              p.ISPType,
		EncryptedCredentials: encCreds,
		Transports:           p.Transports,
		IsDefault:            p.IsDefault,
		Options: models.ISPProfileOptions{
			UsaPegarFaturaAtrasada:      p.Options.UsaPegarFaturaAtrasada,
			UsaExtrairLinhaDigitavelPDF: p.Options.UsaExtrairLinhaDigitavelPDF,
		},
		TimeoutMs: p.TimeoutMs,
		Enabled:   p.Enabled,
	}
	m.ID = p.ID
	m.TenantID = p.TenantID
	m.CreatedAt = p.CreatedAt
	m.UpdatedAt = p.UpdatedAt
	return m, nil
}

func (r *ProfileRepository) toEntity(m *models.ISPProfile) (*entity.ISPProfile, error) {
	var creds map[string]string
	if m.EncryptedCredentials != "" {
		raw, err := r.cipher.Decrypt(m.EncryptedCredentials)
		if err != nil {
			return nil, apperror.Internal("decrypt credentials").Wrap(err)
		}
		if err := json.Unmarshal([]byte(raw), &creds); err != nil {
			return nil, apperror.Internal("unmarshal credentials").Wrap(err)
		}
	}
	return &entity.ISPProfile{
		ID:          m.ID,
		TenantID:    m.TenantID,
		Label:       m.Label,
		ISPType:     m.ISPType,
		Credentials: creds,
		Transports:  m.Transports,
		IsDefault:   m.IsDefault,
		Options: entity.Options{
			UsaPegarFaturaAtrasada:      m.Options.UsaPegarFaturaAtrasada,
			UsaExtrairLinhaDigitavelPDF: m.Options.UsaExtrairLinhaDigitavelPDF,
		},
		TimeoutMs: m.TimeoutMs,
		Enabled:   m.Enabled,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

var _ repository.ProfileRepository = (*ProfileRepository)(nil)
