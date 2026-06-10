package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/apperror"
	chentity "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/entity"
	chrepo "github.com/romerito007/chat-smsnet-omnichannel/domain/channels/repository"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

type fakeIntegrationRepo struct {
	chrepo.IntegrationRepository
	byKey map[string]*chentity.Integration
}

func (r *fakeIntegrationRepo) FindByIntegrationKey(_ context.Context, key string) (*chentity.Integration, error) {
	if i, ok := r.byKey[key]; ok {
		return i, nil
	}
	return nil, apperror.NotFound("not found")
}

func sign(body, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func newChannelService(i *chentity.Integration) *ChannelService {
	repo := &fakeIntegrationRepo{byKey: map[string]*chentity.Integration{i.IntegrationKey: i}}
	return NewChannelService(repo, shared.SystemClock{})
}

func baseIntegration() *chentity.Integration {
	return &chentity.Integration{
		ID: "i1", TenantID: "t1", Channel: "whatsapp",
		IntegrationKey: "pubkey", Secret: "s3cr3t", Enabled: true,
	}
}

func TestAuthenticate_ValidHMAC(t *testing.T) {
	svc := newChannelService(baseIntegration())
	body := `{"external_message_id":"x"}`
	got, err := svc.Authenticate(context.Background(), "pubkey", "whatsapp", body, "sha256="+sign(body, "s3cr3t"), "")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got.TenantID != "t1" {
		t.Errorf("tenant from integration = %q", got.TenantID)
	}
}

func TestAuthenticate_InvalidHMAC(t *testing.T) {
	svc := newChannelService(baseIntegration())
	body := `{"external_message_id":"x"}`
	if _, err := svc.Authenticate(context.Background(), "pubkey", "whatsapp", body, "sha256="+sign(body, "wrong"), ""); apperror.From(err).Code != apperror.CodeUnauthorized {
		t.Errorf("expected unauthorized for bad signature, got %v", err)
	}
}

func TestAuthenticate_SecretHeader(t *testing.T) {
	svc := newChannelService(baseIntegration())
	if _, err := svc.Authenticate(context.Background(), "pubkey", "whatsapp", "{}", "", "s3cr3t"); err != nil {
		t.Errorf("valid secret should authenticate: %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), "pubkey", "whatsapp", "{}", "", "nope"); apperror.From(err).Code != apperror.CodeUnauthorized {
		t.Errorf("invalid secret should be unauthorized, got %v", err)
	}
}

func TestAuthenticate_MissingSignature(t *testing.T) {
	svc := newChannelService(baseIntegration())
	if _, err := svc.Authenticate(context.Background(), "pubkey", "whatsapp", "{}", "", ""); apperror.From(err).Code != apperror.CodeUnauthorized {
		t.Errorf("expected unauthorized without signature/secret, got %v", err)
	}
}

func TestAuthenticate_UnknownIntegration(t *testing.T) {
	svc := newChannelService(baseIntegration())
	if _, err := svc.Authenticate(context.Background(), "ghost", "whatsapp", "{}", "", "s3cr3t"); apperror.From(err).Code != apperror.CodeUnauthorized {
		t.Errorf("expected unauthorized for unknown key, got %v", err)
	}
}

func TestAuthenticate_ChannelMismatch(t *testing.T) {
	svc := newChannelService(baseIntegration())
	if _, err := svc.Authenticate(context.Background(), "pubkey", "telegram", "{}", "", "s3cr3t"); apperror.From(err).Code != apperror.CodeUnauthorized {
		t.Errorf("expected unauthorized for channel mismatch, got %v", err)
	}
}

func TestAuthenticate_DisabledIntegration(t *testing.T) {
	i := baseIntegration()
	i.Enabled = false
	svc := newChannelService(i)
	if _, err := svc.Authenticate(context.Background(), "pubkey", "whatsapp", "{}", "", "s3cr3t"); apperror.From(err).Code != apperror.CodeUnauthorized {
		t.Errorf("expected unauthorized for disabled integration, got %v", err)
	}
}
