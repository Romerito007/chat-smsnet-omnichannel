package migrations

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/romerito007/chat-smsnet-omnichannel/domain/shared"
)

// 0030 migrates the legacy single ProviderHub config (providerhub_configs, one
// enabled per tenant) into one isp_profiles document with is_default=true,
// preserving the current behavior. The SMSNET gateway host/key are intentionally
// dropped here — they become infra-only (env ISP_GATEWAY_API_HOST/KEY). The
// encrypted credentials are copied verbatim (same cipher), so nothing is
// decrypted during the migration.
//
// Idempotent: a tenant that already has any default profile is skipped, so
// re-running (or running after a tenant created profiles manually) is a no-op.
func init() {
	Register(Migration{
		Version: 30,
		Name:    "migrate_single_config_to_profile",
		Up: func(ctx context.Context, db *mongo.Database) error {
			configs := db.Collection("providerhub_configs")
			profiles := db.Collection("isp_profiles")

			cur, err := configs.Find(ctx, bson.M{"enabled": true})
			if err != nil {
				return err
			}
			defer func() { _ = cur.Close(ctx) }()

			now := time.Now().UTC()
			for cur.Next(ctx) {
				var c struct {
					TenantID             string `bson:"tenant_id"`
					Name                 string `bson:"name"`
					ISPType              string `bson:"isp_type"`
					EncryptedCredentials string `bson:"encrypted_credentials"`
					Options              struct {
						UsaPegarFaturaAtrasada      bool `bson:"usa_pegar_fatura_atrasada"`
						UsaExtrairLinhaDigitavelPDF bool `bson:"usa_extrair_linha_digitavel_pdf"`
					} `bson:"options"`
					TimeoutMs int `bson:"timeout_ms"`
				}
				if err := cur.Decode(&c); err != nil {
					return err
				}
				if c.TenantID == "" || c.ISPType == "" {
					continue
				}
				// Skip tenants that already have a default profile (idempotent re-run).
				existing, err := profiles.CountDocuments(ctx, bson.M{"tenant_id": c.TenantID, "is_default": true})
				if err != nil {
					return err
				}
				if existing > 0 {
					continue
				}
				label := c.Name
				if label == "" {
					label = "Perfil ISP"
				}
				timeout := c.TimeoutMs
				if timeout <= 0 {
					timeout = 8000
				}
				doc := bson.M{
					"_id":                   shared.NewID(),
					"tenant_id":             c.TenantID,
					"label":                 label,
					"isp_type":              c.ISPType,
					"encrypted_credentials": c.EncryptedCredentials,
					"is_default":            true,
					"options": bson.M{
						"usa_pegar_fatura_atrasada":       c.Options.UsaPegarFaturaAtrasada,
						"usa_extrair_linha_digitavel_pdf": c.Options.UsaExtrairLinhaDigitavelPDF,
					},
					"timeout_ms": timeout,
					"enabled":    true,
					"created_at": now,
					"updated_at": now,
				}
				if _, err := profiles.InsertOne(ctx, doc); err != nil {
					return err
				}
			}
			return cur.Err()
		},
	})
}
