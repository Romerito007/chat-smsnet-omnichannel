// Package service holds the channels business logic: connection management,
// inbound authentication/orchestration and outbound delivery.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ConnectionService manages channel connections and authenticates inbound calls.
type ConnectionService struct {
	repo     repository.ConnectionRepository
	registry contracts.AdapterRegistry
	clock    shared.Clock
}

// NewConnectionService builds the service.
func NewConnectionService(repo repository.ConnectionRepository, registry contracts.AdapterRegistry, clock shared.Clock) *ConnectionService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &ConnectionService{repo: repo, registry: registry, clock: clock}
}

// Create registers a connection, generating its webhook verify token.
func (s *ConnectionService) Create(ctx context.Context, cmd contracts.CreateConnection) (*entity.ChannelConnection, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !cmd.Type.Valid() {
		return nil, apperror.Validation("invalid channel type").
			WithDetails(map[string]any{"type": "must be whatsapp|telegram|instagram|webchat|custom"})
	}
	authType := cmd.AuthType
	if authType == "" {
		authType = entity.AuthToken
	}
	now := s.clock.Now()
	conn := &entity.ChannelConnection{
		ID:                 shared.NewID(),
		TenantID:           tenantID,
		Type:               cmd.Type,
		Name:               strings.TrimSpace(cmd.Name),
		Status:             entity.StatusDisconnected,
		BaseURL:            strings.TrimSpace(cmd.BaseURL),
		AuthType:           authType,
		Secret:             cmd.Secret,
		WebhookVerifyToken: randomToken(24),
		DefaultSectorID:    cmd.DefaultSectorID,
		Enabled:            true,
		AutomationEnabled:  cmd.AutomationEnabled,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.repo.Create(ctx, conn); err != nil {
		return nil, err
	}
	return conn, nil
}

// Update applies the non-nil fields of cmd.
func (s *ConnectionService) Update(ctx context.Context, id string, cmd contracts.UpdateConnection) (*entity.ChannelConnection, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	conn, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		conn.Name = strings.TrimSpace(*cmd.Name)
	}
	if cmd.Status != nil {
		conn.Status = *cmd.Status
	}
	if cmd.BaseURL != nil {
		conn.BaseURL = strings.TrimSpace(*cmd.BaseURL)
	}
	if cmd.AuthType != nil {
		conn.AuthType = *cmd.AuthType
	}
	if cmd.Secret != nil {
		conn.Secret = *cmd.Secret
	}
	if cmd.DefaultSectorID != nil {
		conn.DefaultSectorID = *cmd.DefaultSectorID
	}
	if cmd.Enabled != nil {
		conn.Enabled = *cmd.Enabled
	}
	if cmd.AutomationEnabled != nil {
		conn.AutomationEnabled = *cmd.AutomationEnabled
	}
	conn.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, conn); err != nil {
		return nil, err
	}
	return conn, nil
}

// Delete removes a connection.
func (s *ConnectionService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Get returns a connection by id.
func (s *ConnectionService) Get(ctx context.Context, id string) (*entity.ChannelConnection, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns a page of connections.
func (s *ConnectionService) List(ctx context.Context, page shared.PageRequest) ([]*entity.ChannelConnection, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, page.Normalize())
}

// Test exercises the connection by sending a probe message through its adapter,
// updating the connection status accordingly. It never errors on a channel
// failure — it reports the outcome.
func (s *ConnectionService) Test(ctx context.Context, id string) (contracts.TestResult, *entity.ChannelConnection, error) {
	conn, err := s.Get(ctx, id)
	if err != nil {
		return contracts.TestResult{}, nil, err
	}
	adapter := s.registry.For(conn.Type)
	if adapter == nil {
		return contracts.TestResult{OK: false, Error: "no adapter for channel type"}, conn, nil
	}

	res, err := adapter.SendMessage(ctx, conn, contracts.OutboundSend{
		ExternalContactID: "connection-test",
		Text:              "connection test",
	})
	now := s.clock.Now()
	if err != nil {
		conn.Status = entity.StatusError
		conn.UpdatedAt = now
		_ = s.repo.Update(ctx, conn)
		return contracts.TestResult{OK: false, Error: err.Error()}, conn, nil
	}
	conn.Status = entity.StatusConnected
	conn.UpdatedAt = now
	if err := s.repo.Update(ctx, conn); err != nil {
		return contracts.TestResult{}, nil, err
	}
	return contracts.TestResult{OK: true, ExternalMessageID: res.ExternalMessageID}, conn, nil
}

// ResolveInbound resolves and verifies an inbound request/receipt by its webhook
// verify token, returning the (tenant-bearing) connection. Verification is
// delegated to the channel adapter.
func (s *ConnectionService) ResolveInbound(ctx context.Context, token string, channelType entity.Type, rawBody []byte, headers map[string]string) (*entity.ChannelConnection, error) {
	if token == "" {
		return nil, apperror.Unauthorized("missing webhook token")
	}
	conn, err := s.repo.FindByWebhookVerifyToken(ctx, token)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return nil, apperror.Unauthorized("unknown channel")
		}
		return nil, err
	}
	if !conn.Enabled {
		return nil, apperror.Unauthorized("channel is disabled")
	}
	if conn.Type != channelType {
		return nil, apperror.Unauthorized("channel type mismatch")
	}
	adapter := s.registry.For(conn.Type)
	if adapter == nil {
		return nil, apperror.Unauthorized("no adapter for channel type")
	}
	if err := adapter.VerifyInbound(conn, rawBody, headers); err != nil {
		return nil, apperror.Unauthorized("inbound verification failed")
	}
	return conn, nil
}

func randomToken(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
