// Package service holds the channels business logic: integration management and
// inbound authentication, plus the inbound message orchestration.
package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// ChannelService manages channel integrations and authenticates inbound calls.
type ChannelService struct {
	repo  repository.IntegrationRepository
	clock shared.Clock
}

// NewChannelService builds the service.
func NewChannelService(repo repository.IntegrationRepository, clock shared.Clock) *ChannelService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &ChannelService{repo: repo, clock: clock}
}

// Create registers an integration, generating its public key and secret.
func (s *ChannelService) Create(ctx context.Context, cmd contracts.CreateIntegration) (*entity.Integration, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	channel := strings.TrimSpace(cmd.Channel)
	if channel == "" {
		return nil, apperror.Validation("channel is required").WithDetails(map[string]any{"channel": "is required"})
	}
	now := s.clock.Now()
	integration := &entity.Integration{
		ID:                shared.NewID(),
		TenantID:          tenantID,
		Channel:           channel,
		Name:              strings.TrimSpace(cmd.Name),
		IntegrationKey:    randomToken(16),
		Secret:            randomToken(32),
		Enabled:           true,
		AutomationEnabled: cmd.AutomationEnabled,
		DefaultQueueID:    cmd.DefaultQueueID,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.repo.Create(ctx, integration); err != nil {
		return nil, err
	}
	return integration, nil
}

// Get returns an integration by id.
func (s *ChannelService) Get(ctx context.Context, id string) (*entity.Integration, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns a page of integrations.
func (s *ChannelService) List(ctx context.Context, page shared.PageRequest) ([]*entity.Integration, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, page.Normalize())
}

// Authenticate resolves and verifies an inbound request. The integration is
// looked up by its key (pre-auth); the channel must match the path; and the
// request must carry a valid HMAC signature (preferred) or the exact secret.
//
// The returned integration carries the authoritative tenant id.
func (s *ChannelService) Authenticate(ctx context.Context, integrationKey, channel, rawBody, signature, secretHeader string) (*entity.Integration, error) {
	if integrationKey == "" {
		return nil, apperror.Unauthorized("missing integration key")
	}
	integration, err := s.repo.FindByIntegrationKey(ctx, integrationKey)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return nil, apperror.Unauthorized("unknown integration")
		}
		return nil, err
	}
	if !integration.Enabled {
		return nil, apperror.Unauthorized("integration is disabled")
	}
	if integration.Channel != channel {
		return nil, apperror.Unauthorized("integration channel mismatch")
	}

	if signature != "" {
		if !validHMAC(rawBody, integration.Secret, signature) {
			return nil, apperror.Unauthorized("invalid signature")
		}
		return integration, nil
	}
	if secretHeader != "" {
		if subtle.ConstantTimeCompare([]byte(secretHeader), []byte(integration.Secret)) == 1 {
			return integration, nil
		}
		return nil, apperror.Unauthorized("invalid secret")
	}
	return nil, apperror.Unauthorized("missing signature")
}

// validHMAC verifies a hex HMAC-SHA256 signature (optionally "sha256="-prefixed)
// of the raw body keyed by the secret.
func validHMAC(body, secret, signature string) bool {
	signature = strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(signature))) == 1
}

func randomToken(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
