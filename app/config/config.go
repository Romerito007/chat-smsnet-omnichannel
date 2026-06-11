// Package config loads runtime configuration from environment variables (with
// optional .env support) into a single immutable struct shared by every role.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Role selects which routines the binary boots. A single binary runs every
// role; RUN_ROLE picks the active one.
type Role string

const (
	RoleAll       Role = "all"
	RoleAPI       Role = "api"
	RoleWS        Role = "ws"
	RoleWorker    Role = "worker"
	RoleScheduler Role = "scheduler"
)

// Config is the fully resolved configuration for a process.
type Config struct {
	AppEnv   string // development | staging | production
	LogLevel string
	RunRole  Role

	HTTP          HTTPConfig
	Mongo         MongoConfig
	Redis         RedisConfig
	Asynq         AsynqConfig
	Otel          OtelConfig
	Auth          AuthConfig
	Realtime      RealtimeConfig
	Channels      ChannelsConfig
	Automation    AutomationConfig
	ProviderHub   ProviderHubConfig
	Copilot       CopilotConfig
	Notifications NotificationsConfig
	CSAT          CSATConfig
	Maintenance   MaintenanceConfig
	Privacy       PrivacyConfig
	Attachments   AttachmentsConfig

	// Seed identifies the bootstrap tenant/owner created on first run.
	Seed SeedConfig
}

// MaintenanceConfig holds the default limits for the periodic jobs. Each can be
// overridden per tenant via tenant settings.
type MaintenanceConfig struct {
	// InactiveCloseAfter is the idle time after which an open conversation is
	// auto-closed (tenant override: settings.inactive_close_after_minutes).
	InactiveCloseAfter time.Duration
	// NotificationRetention is how long read notifications are kept
	// (tenant override: settings.notification_retention_days).
	NotificationRetention time.Duration
	// AuditRetention is how long audit logs are kept
	// (tenant override: settings.audit_retention_days).
	AuditRetention time.Duration
}

// AttachmentsConfig holds the attachments settings: storage backend selection
// (local | s3), upload validation and signed-URL lifetimes.
type AttachmentsConfig struct {
	// Provider selects the storage backend: "local" or "s3".
	Provider string
	// MaxSizeBytes caps an uploaded attachment.
	MaxSizeBytes int64
	// AllowedContentTypes is the MIME allow-list (supports "image/*"); empty
	// allows any.
	AllowedContentTypes []string
	// UploadTTL / DownloadTTL bound the signed upload and download URLs.
	UploadTTL   time.Duration
	DownloadTTL time.Duration
	// SigningSecret signs local upload tokens.
	SigningSecret string
	// LocalDir is the base directory for the local backend.
	LocalDir string
	// BaseURL is the public API origin used to build upload/download URLs.
	BaseURL string
	// S3 holds the S3-compatible backend settings (used when Provider == "s3").
	S3 AttachmentsS3Config
}

// AttachmentsS3Config holds S3-compatible backend settings.
type AttachmentsS3Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

// PrivacyConfig holds the privacy (LGPD) settings: where export files are stored
// and how their temporary signed download URLs are minted.
type PrivacyConfig struct {
	// StorageDir is the base directory for export artifacts.
	StorageDir string
	// SigningSecret signs download tokens (HMAC). Set a strong value in production.
	SigningSecret string
	// DownloadBaseURL is the public API origin the signed link points at.
	DownloadBaseURL string
	// DownloadTTL bounds how long an export's signed URL stays valid.
	DownloadTTL time.Duration
}

// CSATConfig holds the CSAT settings.
type CSATConfig struct {
	// ExpireAfterSeconds bounds how long a sent survey waits for an answer.
	ExpireAfterSeconds int
	// PublicBaseURL is the base for the public answer link sent to customers.
	PublicBaseURL string
}

// NotificationsConfig holds the notifications settings.
type NotificationsConfig struct {
	// EmailFrom is the From address for notification emails.
	EmailFrom string
	// AppBaseURL is the base for deep links embedded in notifications.
	AppBaseURL string
}

// AuthConfig holds the JWT and password settings.
type AuthConfig struct {
	JWTSecret  string
	Issuer     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	BcryptCost int
}

// HTTPConfig holds the HTTP/WS server settings.
type HTTPConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	AllowedOrigins  []string
}

// MongoConfig holds MongoDB connection settings.
type MongoConfig struct {
	URI      string
	Database string
}

// RedisConfig holds Redis connection settings (shared by cache, pub/sub and
// Asynq).
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// AsynqConfig holds the worker concurrency and queue priorities.
type AsynqConfig struct {
	Concurrency int
	Queues      map[string]int
}

// OtelConfig holds OpenTelemetry exporter settings.
type OtelConfig struct {
	Enabled     bool
	ServiceName string
}

// RealtimeConfig holds the WebSocket settings.
type RealtimeConfig struct {
	// MaxConnPerUser bounds simultaneous WS connections per user (0 = unlimited).
	MaxConnPerUser int
}

// ChannelsConfig holds the channels domain settings.
type ChannelsConfig struct {
	// EncryptionKey encrypts channel credentials at rest. Set a strong value in
	// production.
	EncryptionKey string
}

// AutomationConfig holds the automation domain settings.
type AutomationConfig struct {
	// CallbackBaseURL is the public base URL the external flow uses to call back.
	CallbackBaseURL string
}

// ProviderHubConfig holds the providerhub settings.
type ProviderHubConfig struct {
	// RatePerMinute caps outbound provider queries per tenant per minute.
	RatePerMinute int
}

// CopilotConfig holds the copilot provider API keys. The MVP uses the echo mock
// (no key needed); a hosted provider is activated only when its key is set.
type CopilotConfig struct {
	OpenAIKey    string
	GeminiKey    string
	AnthropicKey string
}

// SeedConfig holds the idempotent first-run seed identity.
type SeedConfig struct {
	TenantName    string
	OwnerEmail    string
	OwnerName     string
	OwnerPassword string
}

// Load reads configuration from the environment. If an .env file exists it is
// loaded first (without overriding already-set variables).
func Load() (Config, error) {
	_ = godotenv.Load() // best-effort; real env always wins

	cfg := Config{
		AppEnv:   getString("APP_ENV", "development"),
		LogLevel: getString("LOG_LEVEL", "info"),
		RunRole:  Role(getString("RUN_ROLE", string(RoleAll))),
		HTTP: HTTPConfig{
			Port:            getInt("HTTP_PORT", 8080),
			ReadTimeout:     getDuration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    getDuration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getDuration("HTTP_SHUTDOWN_TIMEOUT", 20*time.Second),
			AllowedOrigins:  getList("HTTP_ALLOWED_ORIGINS", []string{"*"}),
		},
		Mongo: MongoConfig{
			URI:      getString("MONGO_URI", "mongodb://localhost:27017"),
			Database: getString("MONGO_DATABASE", "chat"),
		},
		Redis: RedisConfig{
			Addr:     getString("REDIS_ADDR", "localhost:6379"),
			Password: getString("REDIS_PASSWORD", ""),
			DB:       getInt("REDIS_DB", 0),
		},
		Asynq: AsynqConfig{
			Concurrency: getInt("ASYNQ_CONCURRENCY", 20),
			// Priorities mirror section 4 of the architecture doc.
			Queues: map[string]int{
				"critical": getInt("ASYNQ_QUEUE_CRITICAL", 6),
				"default":  getInt("ASYNQ_QUEUE_DEFAULT", 3),
				"channels": getInt("ASYNQ_QUEUE_CHANNELS", 3),
				"webhooks": getInt("ASYNQ_QUEUE_WEBHOOKS", 2),
				"ai":       getInt("ASYNQ_QUEUE_AI", 2),
				"reports":  getInt("ASYNQ_QUEUE_REPORTS", 1),
			},
		},
		Otel: OtelConfig{
			Enabled:     getBool("OTEL_ENABLED", false),
			ServiceName: getString("OTEL_SERVICE_NAME", "chat-backend"),
		},
		Auth: AuthConfig{
			JWTSecret:  getString("AUTH_JWT_SECRET", "dev-secret-change-me"),
			Issuer:     getString("AUTH_ISSUER", "chat-backend"),
			AccessTTL:  getDuration("AUTH_ACCESS_TTL", 15*time.Minute),
			RefreshTTL: getDuration("AUTH_REFRESH_TTL", 720*time.Hour),
			BcryptCost: getInt("AUTH_BCRYPT_COST", 12),
		},
		Realtime: RealtimeConfig{
			MaxConnPerUser: getInt("WS_MAX_CONN_PER_USER", 10),
		},
		Channels: ChannelsConfig{
			EncryptionKey: getString("CHANNELS_ENCRYPTION_KEY", "dev-channel-encryption-key"),
		},
		Automation: AutomationConfig{
			CallbackBaseURL: getString("AUTOMATION_CALLBACK_BASE_URL", "http://localhost:8080"),
		},
		ProviderHub: ProviderHubConfig{
			RatePerMinute: getInt("PROVIDERHUB_RATE_PER_MINUTE", 60),
		},
		Copilot: CopilotConfig{
			OpenAIKey:    getString("COPILOT_OPENAI_API_KEY", ""),
			GeminiKey:    getString("COPILOT_GEMINI_API_KEY", ""),
			AnthropicKey: getString("COPILOT_ANTHROPIC_API_KEY", ""),
		},
		Notifications: NotificationsConfig{
			EmailFrom:  getString("NOTIFICATIONS_EMAIL_FROM", "no-reply@example.com"),
			AppBaseURL: getString("APP_BASE_URL", "http://localhost:3000"),
		},
		CSAT: CSATConfig{
			ExpireAfterSeconds: getInt("CSAT_EXPIRE_AFTER_SECONDS", 72*3600),
			PublicBaseURL:      getString("CSAT_PUBLIC_BASE_URL", getString("APP_BASE_URL", "http://localhost:3000")),
		},
		Maintenance: MaintenanceConfig{
			InactiveCloseAfter:    getDuration("MAINTENANCE_INACTIVE_CLOSE_AFTER", 24*time.Hour),
			NotificationRetention: getDuration("MAINTENANCE_NOTIFICATION_RETENTION", 30*24*time.Hour),
			AuditRetention:        getDuration("MAINTENANCE_AUDIT_RETENTION", 365*24*time.Hour),
		},
		Privacy: PrivacyConfig{
			StorageDir:      getString("PRIVACY_STORAGE_DIR", "/tmp/chat-exports"),
			SigningSecret:   getString("PRIVACY_SIGNING_SECRET", getString("AUTH_JWT_SECRET", "dev-secret-change-me")),
			DownloadBaseURL: getString("PRIVACY_DOWNLOAD_BASE_URL", "http://localhost:8080"),
			DownloadTTL:     getDuration("PRIVACY_DOWNLOAD_TTL", 24*time.Hour),
		},
		Attachments: AttachmentsConfig{
			Provider:            getString("ATTACHMENTS_PROVIDER", "local"),
			MaxSizeBytes:        int64(getInt("ATTACHMENTS_MAX_SIZE_BYTES", 25<<20)),
			AllowedContentTypes: getList("ATTACHMENTS_ALLOWED_CONTENT_TYPES", nil),
			UploadTTL:           getDuration("ATTACHMENTS_UPLOAD_TTL", 15*time.Minute),
			DownloadTTL:         getDuration("ATTACHMENTS_DOWNLOAD_TTL", 5*time.Minute),
			SigningSecret:       getString("ATTACHMENTS_SIGNING_SECRET", getString("AUTH_JWT_SECRET", "dev-secret-change-me")),
			LocalDir:            getString("ATTACHMENTS_LOCAL_DIR", "/tmp/chat-attachments"),
			BaseURL:             getString("ATTACHMENTS_BASE_URL", "http://localhost:8080"),
			S3: AttachmentsS3Config{
				Endpoint:  getString("ATTACHMENTS_S3_ENDPOINT", ""),
				Region:    getString("ATTACHMENTS_S3_REGION", "us-east-1"),
				Bucket:    getString("ATTACHMENTS_S3_BUCKET", ""),
				AccessKey: getString("ATTACHMENTS_S3_ACCESS_KEY", ""),
				SecretKey: getString("ATTACHMENTS_S3_SECRET_KEY", ""),
			},
		},
		Seed: SeedConfig{
			TenantName:    getString("SEED_TENANT_NAME", "Default Tenant"),
			OwnerEmail:    getString("SEED_OWNER_EMAIL", "owner@example.com"),
			OwnerName:     getString("SEED_OWNER_NAME", "Owner"),
			OwnerPassword: getString("SEED_OWNER_PASSWORD", "change-me-now"),
		},
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	switch c.RunRole {
	case RoleAll, RoleAPI, RoleWS, RoleWorker, RoleScheduler:
	default:
		return fmt.Errorf("invalid RUN_ROLE %q", c.RunRole)
	}
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		return fmt.Errorf("invalid HTTP_PORT %d", c.HTTP.Port)
	}
	if c.Mongo.URI == "" {
		return fmt.Errorf("MONGO_URI is required")
	}
	if c.AppEnv == "production" && c.Auth.JWTSecret == "dev-secret-change-me" {
		return fmt.Errorf("AUTH_JWT_SECRET must be set in production")
	}
	return nil
}

// RunsRole reports whether the active configuration should boot the given role.
func (c Config) RunsRole(r Role) bool {
	return c.RunRole == RoleAll || c.RunRole == r
}

func getString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func getBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			return d
		}
	}
	return def
}

func getList(key string, def []string) []string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return def
}
