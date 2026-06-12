// Package service holds the channels business logic: connection management,
// inbound authentication/orchestration and outbound delivery.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"math/big"
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
	health   contracts.ConnectionHealthChecker
	auditor  shared.Auditor
}

// NewConnectionService builds the service.
func NewConnectionService(repo repository.ConnectionRepository, registry contracts.AdapterRegistry, clock shared.Clock) *ConnectionService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &ConnectionService{repo: repo, registry: registry, clock: clock, health: contracts.NoopHealthChecker{}, auditor: shared.NoopAuditor{}}
}

// SetAuditor wires the audit trail. Optional: when unset, token rotations are
// not recorded.
func (s *ConnectionService) SetAuditor(a shared.Auditor) {
	if a != nil {
		s.auditor = a
	}
}

// SetHealthChecker wires the connection health checker. Optional: when unset,
// connections are treated as healthy.
func (s *ConnectionService) SetHealthChecker(h contracts.ConnectionHealthChecker) {
	if h != nil {
		s.health = h
	}
}

// HealthCheck probes the current tenant's enabled connections and marks each
// connected/error based on reachability. Idempotent: a connection is only updated
// when its status changes. Returns the number of status changes. Run by the
// channels.health_check job.
func (s *ConnectionService) HealthCheck(ctx context.Context) (int, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return 0, err
	}
	conns, err := s.repo.List(ctx, shared.PageRequest{Limit: shared.MaxPageSize})
	if err != nil {
		return 0, err
	}
	changed := 0
	now := s.clock.Now()
	for _, conn := range conns {
		if !conn.Enabled {
			continue
		}
		newStatus := entity.StatusConnected
		if err := s.health.Check(ctx, conn); err != nil {
			newStatus = entity.StatusError
		}
		if conn.Status == newStatus {
			continue
		}
		conn.Status = newStatus
		conn.UpdatedAt = now
		if err := s.repo.Update(ctx, conn); err != nil {
			continue
		}
		changed++
	}
	return changed, nil
}

// Create registers a connection, generating its integration token (returned in
// plaintext once via InboundToken; only the hash is persisted).
func (s *ConnectionService) Create(ctx context.Context, cmd contracts.CreateConnection) (*entity.ChannelConnection, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if !cmd.Type.Valid() {
		return nil, apperror.Validation("invalid channel type").
			WithDetails(map[string]any{"type": "must be api|whatsapp|telegram|instagram|webchat|custom"})
	}
	authType := cmd.AuthType
	if authType == "" {
		authType = entity.AuthToken
	}
	// The API channel signs outbound deliveries with an HMAC secret. When the
	// company does not supply one, generate it so a signing key always exists;
	// it is returned once on creation and never again.
	secret := cmd.Secret
	if cmd.Type == entity.TypeAPI && secret == "" {
		secret = randomToken(32)
	}
	now := s.clock.Now()
	token := newInboundToken()
	conn := &entity.ChannelConnection{
		ID:                shared.NewID(),
		TenantID:          tenantID,
		Type:              cmd.Type,
		Name:              strings.TrimSpace(cmd.Name),
		Status:            entity.StatusDisconnected,
		BaseURL:           strings.TrimSpace(cmd.BaseURL),
		AuthType:          authType,
		Secret:            secret,
		InboundToken:      token,
		InboundTokenHash:  hashInboundToken(token),
		DefaultSectorID:   cmd.DefaultSectorID,
		Enabled:           true,
		AutomationEnabled: cmd.AutomationEnabled,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.repo.Create(ctx, conn); err != nil {
		return nil, err
	}
	return conn, nil
}

// RotateInboundToken issues a fresh integration token for the channel, revoking
// the previous one (only the new hash is stored). The plaintext is returned once
// on the entity's InboundToken field; thereafter only has_inbound_token is shown.
// Audited as channel.token_rotated.
func (s *ConnectionService) RotateInboundToken(ctx context.Context, id string) (*entity.ChannelConnection, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	token := newInboundToken()
	conn.InboundToken = token
	conn.InboundTokenHash = hashInboundToken(token)
	conn.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, conn); err != nil {
		return nil, err
	}
	_ = s.auditor.Record(ctx, shared.AuditEntry{
		TenantID:     tenantID,
		Action:       "channel.token_rotated",
		ResourceType: "channel",
		ResourceID:   conn.ID,
		Data:         map[string]any{"type": string(conn.Type)},
	})
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

// ResolveInbound resolves and authenticates an inbound request/receipt by its
// integration token, returning the (tenant-bearing) connection. The token is
// looked up by its hash and re-checked in constant time; the tenant, channel and
// default sector always come from the matched record, never a client header.
// When the channel carries an outbound secret, the adapter additionally verifies
// the HMAC body signature (anti-replay); otherwise the token alone authenticates.
func (s *ConnectionService) ResolveInbound(ctx context.Context, token string, channelType entity.Type, rawBody []byte, headers map[string]string) (*entity.ChannelConnection, error) {
	if token == "" {
		return nil, apperror.Unauthorized("missing inbound token")
	}
	hash := hashInboundToken(token)
	conn, err := s.repo.FindByInboundTokenHash(ctx, hash)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return nil, apperror.Unauthorized("unknown channel")
		}
		return nil, err
	}
	// Defensive constant-time re-check so timing never distinguishes a near-miss.
	if subtle.ConstantTimeCompare([]byte(conn.InboundTokenHash), []byte(hash)) != 1 {
		return nil, apperror.Unauthorized("invalid inbound token")
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

// hashInboundToken returns the SHA-256 hex of an integration token — the only
// form stored and compared, so plaintext is never persisted.
func hashInboundToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// newInboundToken returns a high-entropy, URL-safe base62 integration token
// drawn from 32 random bytes (~190 bits).
func newInboundToken() string { return randomBase62(32) }

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// randomBase62 returns a base62 string carrying at least n bytes of entropy.
func randomBase62(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	// Interpret the bytes as a big integer and emit base62 digits; pad so a
	// leading-zero draw never shortens the token below its entropy budget.
	num := new(big.Int).SetBytes(buf)
	base := big.NewInt(62)
	zero := big.NewInt(0)
	mod := new(big.Int)
	var b strings.Builder
	for num.Cmp(zero) > 0 {
		num.DivMod(num, base, mod)
		b.WriteByte(base62Alphabet[mod.Int64()])
	}
	for b.Len() < 43 { // ceil(32 bytes * 8 / log2(62)) ≈ 43
		b.WriteByte(base62Alphabet[0])
	}
	return b.String()
}

// randomToken returns a hex token of n random bytes (used for the outbound HMAC
// secret).
func randomToken(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
