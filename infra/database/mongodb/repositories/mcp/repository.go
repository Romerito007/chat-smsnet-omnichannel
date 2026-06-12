// Package mcp is the Mongo implementation of the MCP repositories (server
// registrations with encrypted auth tokens, write-action approvals and the
// payload-free call log).
package mcp

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/mcp/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/database/mongodb/models"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/secrets"
)

// ── server repository ─────────────────────────────────────────────────────────

// ServerRepository implements repository.ServerRepository. The auth token is
// encrypted on write and decrypted on read so plaintext is never persisted.
type ServerRepository struct {
	coll   *mongo.Collection
	cipher *secrets.Cipher
}

// NewServerRepository builds the repository.
func NewServerRepository(db *mongo.Database, cipher *secrets.Cipher) *ServerRepository {
	return &ServerRepository{coll: db.Collection("mcp_servers"), cipher: cipher}
}

func (r *ServerRepository) Create(ctx context.Context, s *entity.ServerConnection) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	m, err := r.toModel(s)
	if err != nil {
		return apperror.Internal("encrypt auth token").Wrap(err)
	}
	_, err = r.coll.InsertOne(ctx, m)
	return mongodb.MapError(err)
}

func (r *ServerRepository) Update(ctx context.Context, s *entity.ServerConnection) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	enc, err := r.cipher.Encrypt(s.AuthToken)
	if err != nil {
		return apperror.Internal("encrypt auth token").Wrap(err)
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": s.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"name":                 s.Name,
			"base_url":             s.BaseURL,
			"auth_header":          s.AuthHeader,
			"encrypted_auth_token": enc,
			"kind":                 string(s.Kind),
			"enabled":              s.Enabled,
			"updated_at":           s.UpdatedAt,
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

func (r *ServerRepository) Delete(ctx context.Context, id string) error {
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

func (r *ServerRepository) FindByID(ctx context.Context, id string) (*entity.ServerConnection, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.McpServer
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return r.toEntity(&m)
}

func (r *ServerRepository) List(ctx context.Context, page shared.PageRequest) ([]*entity.ServerConnection, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := shared.DecodeCursor(page.Cursor)
	if err != nil {
		return nil, err
	}
	filter := mongodb.ApplyKeyset(bson.M{"tenant_id": tenantID}, cur)
	opts := options.Find().SetSort(mongodb.KeysetSort()).SetLimit(int64(page.Limit) + 1)
	return r.find(ctx, filter, opts)
}

func (r *ServerRepository) ListEnabled(ctx context.Context) ([]*entity.ServerConnection, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	return r.find(ctx, bson.M{"tenant_id": tenantID, "enabled": true}, options.Find().SetLimit(100))
}

func (r *ServerRepository) find(ctx context.Context, filter bson.M, opts *options.FindOptions) ([]*entity.ServerConnection, error) {
	c, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var out []*entity.ServerConnection
	for c.Next(ctx) {
		var m models.McpServer
		if err := c.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		e, err := r.toEntity(&m)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, mongodb.MapError(c.Err())
}

func (r *ServerRepository) toModel(s *entity.ServerConnection) (models.McpServer, error) {
	enc, err := r.cipher.Encrypt(s.AuthToken)
	if err != nil {
		return models.McpServer{}, err
	}
	m := models.McpServer{
		Name: s.Name, Transport: string(s.Transport), BaseURL: s.BaseURL,
		AuthHeader: s.AuthHeader, EncryptedAuthToken: enc, Kind: string(s.Kind), Enabled: s.Enabled,
	}
	m.ID = s.ID
	m.TenantID = s.TenantID
	m.CreatedAt = s.CreatedAt
	m.UpdatedAt = s.UpdatedAt
	return m, nil
}

func (r *ServerRepository) toEntity(m *models.McpServer) (*entity.ServerConnection, error) {
	token, err := r.cipher.Decrypt(m.EncryptedAuthToken)
	if err != nil {
		return nil, apperror.Internal("decrypt auth token").Wrap(err)
	}
	return &entity.ServerConnection{
		ID: m.ID, TenantID: m.TenantID, Name: m.Name, Transport: entity.Transport(m.Transport),
		BaseURL: m.BaseURL, AuthHeader: m.AuthHeader, AuthToken: token, Kind: entity.Kind(m.Kind),
		Enabled: m.Enabled, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}, nil
}

var _ repository.ServerRepository = (*ServerRepository)(nil)

// ── approval repository ───────────────────────────────────────────────────────

// ApprovalRepository implements repository.ApprovalRepository.
type ApprovalRepository struct{ coll *mongo.Collection }

// NewApprovalRepository builds the repository.
func NewApprovalRepository(db *mongo.Database) *ApprovalRepository {
	return &ApprovalRepository{coll: db.Collection("mcp_approvals")}
}

func (r *ApprovalRepository) Create(ctx context.Context, a *entity.Approval) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	_, err := r.coll.InsertOne(ctx, approvalToModel(a))
	return mongodb.MapError(err)
}

func (r *ApprovalRepository) Update(ctx context.Context, a *entity.Approval) error {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return err
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": a.ID, "tenant_id": tenantID},
		bson.M{"$set": bson.M{
			"status": string(a.Status), "decided_by": a.DecidedBy, "reason": a.Reason,
			"result": a.Result, "error": a.Error, "decided_at": a.DecidedAt,
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

func (r *ApprovalRepository) FindByID(ctx context.Context, id string) (*entity.Approval, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	var m models.McpApproval
	if err := r.coll.FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&m); err != nil {
		return nil, mongodb.MapError(err)
	}
	return approvalToEntity(&m), nil
}

func (r *ApprovalRepository) ListByConversation(ctx context.Context, conversationID string) ([]*entity.Approval, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := r.coll.Find(ctx, bson.M{"tenant_id": tenantID, "conversation_id": conversationID}, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	out := make([]*entity.Approval, 0)
	for cur.Next(ctx) {
		var m models.McpApproval
		if err := cur.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, approvalToEntity(&m))
	}
	return out, mongodb.MapError(cur.Err())
}

func approvalToModel(a *entity.Approval) models.McpApproval {
	return models.McpApproval{
		ID: a.ID, TenantID: a.TenantID, ConversationID: a.ConversationID, ServerID: a.ServerID,
		ServerName: a.ServerName, Tool: a.Tool, Args: a.Args, Status: string(a.Status),
		ProposedBy: a.ProposedBy, DecidedBy: a.DecidedBy, Reason: a.Reason, Result: a.Result,
		Error: a.Error, CreatedAt: a.CreatedAt, DecidedAt: a.DecidedAt,
	}
}

func approvalToEntity(m *models.McpApproval) *entity.Approval {
	return &entity.Approval{
		ID: m.ID, TenantID: m.TenantID, ConversationID: m.ConversationID, ServerID: m.ServerID,
		ServerName: m.ServerName, Tool: m.Tool, Args: m.Args, Status: entity.ApprovalStatus(m.Status),
		ProposedBy: m.ProposedBy, DecidedBy: m.DecidedBy, Reason: m.Reason, Result: m.Result,
		Error: m.Error, CreatedAt: m.CreatedAt, DecidedAt: m.DecidedAt,
	}
}

var _ repository.ApprovalRepository = (*ApprovalRepository)(nil)

// ── call-log repository ───────────────────────────────────────────────────────

// CallLogRepository implements repository.CallLogRepository.
type CallLogRepository struct{ coll *mongo.Collection }

// NewCallLogRepository builds the repository.
func NewCallLogRepository(db *mongo.Database) *CallLogRepository {
	return &CallLogRepository{coll: db.Collection("mcp_call_logs")}
}

func (r *CallLogRepository) Create(ctx context.Context, l *entity.CallLog) error {
	_, err := r.coll.InsertOne(ctx, models.McpCallLog{
		ID: l.ID, TenantID: l.TenantID, UserID: l.UserID, ConversationID: l.ConversationID,
		ServerID: l.ServerID, ServerName: l.ServerName, Tool: l.Tool, Write: l.Write,
		Status: string(l.Status), LatencyMs: l.LatencyMs, ErrorSummary: l.ErrorSummary, CreatedAt: l.CreatedAt,
	})
	return mongodb.MapError(err)
}

func (r *CallLogRepository) ListByConversation(ctx context.Context, conversationID string) ([]*entity.CallLog, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := r.coll.Find(ctx, bson.M{"tenant_id": tenantID, "conversation_id": conversationID}, opts)
	if err != nil {
		return nil, mongodb.MapError(err)
	}
	defer func() { _ = cur.Close(ctx) }()
	out := make([]*entity.CallLog, 0)
	for cur.Next(ctx) {
		var m models.McpCallLog
		if err := cur.Decode(&m); err != nil {
			return nil, mongodb.MapError(err)
		}
		out = append(out, &entity.CallLog{
			ID: m.ID, TenantID: m.TenantID, UserID: m.UserID, ConversationID: m.ConversationID,
			ServerID: m.ServerID, ServerName: m.ServerName, Tool: m.Tool, Write: m.Write,
			Status: entity.CallStatus(m.Status), LatencyMs: m.LatencyMs, ErrorSummary: m.ErrorSummary,
			CreatedAt: m.CreatedAt,
		})
	}
	return out, mongodb.MapError(cur.Err())
}

var _ repository.CallLogRepository = (*CallLogRepository)(nil)
