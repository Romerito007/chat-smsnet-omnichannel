// Package service holds the automation business logic: integration management
// and the run lifecycle (start, callback, decision, timeout).
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
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/contracts"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/automation/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// defaultTimeoutMs is used when an integration sets no timeout.
const defaultTimeoutMs = 30000

// IntegrationService manages automation integrations.
type IntegrationService struct {
	repo  repository.IntegrationRepository
	clock shared.Clock
}

// NewIntegrationService builds the service.
func NewIntegrationService(repo repository.IntegrationRepository, clock shared.Clock) *IntegrationService {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &IntegrationService{repo: repo, clock: clock}
}

// Create registers an integration, generating a secret when none is supplied.
func (s *IntegrationService) Create(ctx context.Context, cmd contracts.CreateIntegration) (*entity.AutomationIntegration, error) {
	tenantID, err := shared.RequireTenant(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cmd.BaseURL) == "" {
		return nil, apperror.Validation("base_url is required").WithDetails(map[string]any{"base_url": "is required"})
	}
	secret := cmd.Secret
	if secret == "" {
		secret = randomToken(32)
	}
	timeout := cmd.TimeoutMs
	if timeout <= 0 {
		timeout = defaultTimeoutMs
	}
	now := s.clock.Now()
	integration := &entity.AutomationIntegration{
		ID:        shared.NewID(),
		TenantID:  tenantID,
		Name:      strings.TrimSpace(cmd.Name),
		BaseURL:   strings.TrimSpace(cmd.BaseURL),
		AuthType:  cmd.AuthType,
		Secret:    secret,
		Enabled:   true,
		TimeoutMs: timeout,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, integration); err != nil {
		return nil, err
	}
	return integration, nil
}

// Update applies the non-nil fields of cmd.
func (s *IntegrationService) Update(ctx context.Context, id string, cmd contracts.UpdateIntegration) (*entity.AutomationIntegration, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	integration, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cmd.Name != nil {
		integration.Name = strings.TrimSpace(*cmd.Name)
	}
	if cmd.BaseURL != nil {
		integration.BaseURL = strings.TrimSpace(*cmd.BaseURL)
	}
	if cmd.AuthType != nil {
		integration.AuthType = *cmd.AuthType
	}
	if cmd.Secret != nil {
		integration.Secret = *cmd.Secret
	}
	if cmd.Enabled != nil {
		integration.Enabled = *cmd.Enabled
	}
	if cmd.TimeoutMs != nil && *cmd.TimeoutMs > 0 {
		integration.TimeoutMs = *cmd.TimeoutMs
	}
	integration.UpdatedAt = s.clock.Now()
	if err := s.repo.Update(ctx, integration); err != nil {
		return nil, err
	}
	return integration, nil
}

// Delete removes an integration.
func (s *IntegrationService) Delete(ctx context.Context, id string) error {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Get returns an integration by id.
func (s *IntegrationService) Get(ctx context.Context, id string) (*entity.AutomationIntegration, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// List returns a page of integrations.
func (s *IntegrationService) List(ctx context.Context, page shared.PageRequest) ([]*entity.AutomationIntegration, error) {
	if _, err := shared.RequireTenant(ctx); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, page.Normalize())
}

// validSignature verifies an HMAC-SHA256 signature of the raw body against the
// integration secret.
func validSignature(secret string, rawBody []byte, signature string) bool {
	if secret == "" {
		return false
	}
	signature = strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(signature))) == 1
}

func randomToken(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
