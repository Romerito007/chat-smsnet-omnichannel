//go:build e2e

package copilot

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/copilot/entity"
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
	return cl.Database("copilot_e2e")
}

// TestConfigAPIKeyEncryptedAtRest verifies the per-tenant API key is stored
// encrypted (never plaintext) and decrypts back on read.
func TestConfigAPIKeyEncryptedAtRest(t *testing.T) {
	db := connect(t)
	_ = db.Drop(context.Background())
	cipher, err := secrets.NewCipher("a-32-byte-or-longer-test-encryption-key")
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	repo := NewConfigRepository(db, cipher)
	ctx := shared.WithTenant(context.Background(), "t-ai")

	cfg := &entity.AIConfig{
		ID: "cfg1", TenantID: "t-ai", Provider: entity.ProviderOpenAI, Model: "gpt-4o-mini",
		APIKey: "sk-super-secret", BaseURL: "https://gw.example/v1", Temperature: 0.5,
		MaxTokens: 256, Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Create(ctx, cfg); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.FindByTenant(ctx)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.APIKey != "sk-super-secret" || got.BaseURL != "https://gw.example/v1" {
		t.Errorf("round-trip mismatch: %+v", got)
	}

	var raw bson.M
	if err := db.Collection("copilot_configs").FindOne(context.Background(), bson.M{"_id": "cfg1"}).Decode(&raw); err != nil {
		t.Fatalf("raw find: %v", err)
	}
	enc, _ := raw["encrypted_api_key"].(string)
	if enc == "" || enc == "sk-super-secret" {
		t.Errorf("api key must be stored encrypted, got %q", enc)
	}
	if _, ok := raw["api_key"]; ok {
		t.Error("plaintext api_key must not be persisted")
	}
}
