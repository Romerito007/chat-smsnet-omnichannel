//go:build e2e

package providerhub

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/providerhub/entity"
	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
	"github.com/romerito007/chat-smsnet-omnichannel/infra/secrets"
)

func connect(t *testing.T) *mongo.Database {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cl, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := cl.Ping(ctx, nil); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return cl.Database("providerhub_e2e")
}

// TestConfigEncryptionRoundTripLive verifies the API key + ISP credentials are
// encrypted at rest (not stored in plaintext) and decrypt back on read.
func TestConfigEncryptionRoundTripLive(t *testing.T) {
	db := connect(t)
	_ = db.Drop(context.Background())
	cipher, err := secrets.NewCipher("a-32-byte-or-longer-test-encryption-key")
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	repo := NewConfigRepository(db, cipher)
	ctx := shared.WithTenant(context.Background(), "t-ph")

	cfg := &entity.ProviderIntegrationConfig{
		ID:            "cfg1",
		TenantID:      "t-ph",
		Name:          "Acme ISP",
		SMSNetBaseURL: "https://smsnet.example",
		SMSNetAPIKey:  "super-secret-key",
		ISPType:       entity.ISPHubsoft,
		ISPCredentials: map[string]string{
			"hubsoft_host":      "https://hubsoft.acme",
			"hubsoft_client_id": "cid-42",
			"hubsoft_secret":    "the-isp-secret",
		},
		BotID:     "bot-7",
		Options:   entity.Options{UsaPegarFaturaAtrasada: true},
		Enabled:   true,
		TimeoutMs: 5000,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Create(ctx, cfg); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Round-trip decrypts back to the originals.
	got, err := repo.FindEnabled(ctx)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.SMSNetAPIKey != "super-secret-key" {
		t.Errorf("api key not round-tripped: %q", got.SMSNetAPIKey)
	}
	if got.ISPCredentials["hubsoft_secret"] != "the-isp-secret" || got.ISPCredentials["hubsoft_client_id"] != "cid-42" {
		t.Errorf("credentials not round-tripped: %+v", got.ISPCredentials)
	}
	if got.ISPType != entity.ISPHubsoft || got.BotID != "bot-7" || !got.Options.UsaPegarFaturaAtrasada {
		t.Errorf("fields not persisted: %+v", got)
	}

	// The raw document stores ciphertext, never the plaintext secrets.
	var raw bson.M
	if err := db.Collection("providerhub_configs").FindOne(context.Background(), bson.M{"_id": "cfg1"}).Decode(&raw); err != nil {
		t.Fatalf("raw find: %v", err)
	}
	encKey, _ := raw["encrypted_api_key"].(string)
	encCreds, _ := raw["encrypted_credentials"].(string)
	if encKey == "" || encKey == "super-secret-key" {
		t.Errorf("api key must be stored encrypted, got %q", encKey)
	}
	if encCreds == "" {
		t.Errorf("credentials must be stored encrypted")
	}
	// No plaintext credential/key field on the document.
	for _, k := range []string{"smsnet_api_key", "isp_credentials", "hubsoft_secret"} {
		if _, ok := raw[k]; ok {
			t.Errorf("plaintext field %q must not be persisted", k)
		}
	}
	t.Logf("encrypted at rest: api_key len=%d, credentials len=%d", len(encKey), len(encCreds))
}
