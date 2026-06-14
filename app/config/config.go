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
	// LogRequestBody logs the (redacted, truncated) request body + error code/message
	// per request. Dev-only diagnostic; default false. Never logs credentials.
	LogRequestBody bool
	RunRole        Role

	HTTP          HTTPConfig
	Mongo         MongoConfig
	Redis         RedisConfig
	Asynq         AsynqConfig
	Otel          OtelConfig
	Auth          AuthConfig
	Platform      PlatformConfig
	Realtime      RealtimeConfig
	Channels      ChannelsConfig
	Automation    AutomationConfig
	ProviderHub   ProviderHubConfig
	MCP           MCPConfig
	Copilot       CopilotConfig
	Notifications NotificationsConfig
	Email         EmailConfig
	CSAT          CSATConfig
	Maintenance   MaintenanceConfig
	Privacy       PrivacyConfig
	Reports       ReportsConfig
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
	// AvatarMaxSizeBytes caps an avatar upload (user/contact). Avatars are
	// always restricted to image/* regardless of AllowedContentTypes.
	AvatarMaxSizeBytes int64
	// AllowedContentTypes is the MIME allow-list (supports "image/*"); empty
	// allows any.
	AllowedContentTypes []string
	// UploadTTL / DownloadTTL bound the signed upload and download URLs.
	UploadTTL   time.Duration
	DownloadTTL time.Duration
	// AvatarURLTTL bounds the short-lived signed avatar URL resolved into
	// Contact/User payloads (loaded directly by the browser, no JWT).
	AvatarURLTTL time.Duration
	// SigningSecret signs local upload tokens.
	SigningSecret string
	// LocalDir is the base directory for the local backend.
	LocalDir string
	// BaseURL is the public API origin used to build upload/download URLs.
	BaseURL string
	// S3 holds the S3-compatible backend settings (used when Provider == "s3").
	S3 AttachmentsS3Config
}

// AttachmentsS3Config holds S3-compatible backend settings. When AccessKey/
// SecretKey are empty, the AWS default credential chain is used (env / shared
// config / IAM role on EC2/ECS).
type AttachmentsS3Config struct {
	Endpoint       string // optional, S3-compatible (MinIO/R2); empty = AWS
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	ForcePathStyle bool
	PresignExpiry  time.Duration
	// EnsureCORS, when true, applies a browser-upload CORS policy to the bucket on
	// startup (best-effort) so direct PUT/GET from the SPA works. Needs the
	// s3:PutBucketCORS permission; failures only warn.
	EnsureCORS bool
	// CORSAllowedOrigins are the browser origins allowed to upload/download
	// directly. Defaults to HTTP_ALLOWED_ORIGINS.
	CORSAllowedOrigins []string
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

// ReportsConfig holds the report-export storage settings: where export files are
// written and how their temporary signed download URLs are minted.
type ReportsConfig struct {
	StorageDir      string
	SigningSecret   string
	DownloadBaseURL string
	DownloadTTL     time.Duration
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

// AuthConfig holds the JWT and password settings, plus the lifetimes of the
// single-use account tokens (email verification, password reset, invitation).
type AuthConfig struct {
	JWTSecret       string
	Issuer          string
	AccessTTL       time.Duration
	RefreshTTL      time.Duration
	BcryptCost      int
	VerificationTTL time.Duration
	ResetTTL        time.Duration
	InviteTTL       time.Duration
}

// PlatformConfig holds the platform-plane service keys used by the external
// provisioner (above tenant isolation). APIKeys maps a key id to the SHA-256 hex
// hash of its secret; the secret itself is never stored. Empty (no keys) disables
// the platform provisioning endpoint.
type PlatformConfig struct {
	APIKeys map[string]string // key_id -> sha256(secret) hex
}

// EmailConfig holds the SMTP transport settings used to send real emails
// (account flows + notification channel). TLSMode is one of "starttls"
// (opportunistic on 587), "tls" (implicit TLS on 465) or "none".
type EmailConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	TLSMode  string
}

// Configured reports whether an SMTP host is set so the sender can refuse to
// silently drop mail.
func (e EmailConfig) Configured() bool { return strings.TrimSpace(e.Host) != "" }

// HTTPConfig holds the HTTP/WS server settings.
type HTTPConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	AllowedOrigins  []string
	// OpenAPIBasicUser/Pass gate GET /openapi.json in production (public in dev).
	OpenAPIBasicUser string
	OpenAPIBasicPass string
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
	// GatewayAPIHost / GatewayAPIKey are the env-default SMSNET Integrations HTTP
	// gateway, used when a tenant has no DB config of its own. The key is read
	// only in the backend and never returned to clients.
	GatewayAPIHost string
	GatewayAPIKey  string
}

// MCPConfig holds the env-default SMSNET MCP server URLs (read/write). A tenant
// DB-registered server overrides these. The MCP hosts run on a private network
// without auth and must never be exposed to the internet.
type MCPConfig struct {
	ConsultasURL string // read tools (no approval)
	OperacoesURL string // write tools (always human-approval)
}

// CopilotConfig holds legacy, environment-level copilot keys. Deprecated: real
// provider credentials are now configured PER TENANT (encrypted at rest) via the
// copilot config endpoint, so these are unused by the provider wiring. Retained
// only to avoid breaking existing .env files.
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
	// Demo data (dev only): when DemoData is true, the boot seeds a rich set of
	// demo entities into the owner's tenant (see app/start_routines/seed_demo.go).
	// DemoReset wipes only the previously demo-seeded docs and recreates them.
	DemoData     bool
	DemoReset    bool
	DemoPassword string
}

// Load reads configuration from the environment. If an .env file exists it is
// loaded first (without overriding already-set variables).
func Load() (Config, error) {
	_ = godotenv.Load() // best-effort; real env always wins

	cfg := Config{
		AppEnv:         getString("APP_ENV", "development"),
		LogLevel:       getString("LOG_LEVEL", "info"),
		LogRequestBody: getBool("LOG_REQUEST_BODY", false),
		RunRole:        Role(getString("RUN_ROLE", string(RoleAll))),
		HTTP: HTTPConfig{
			Port:             getInt("HTTP_PORT", 8080),
			ReadTimeout:      getDuration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:     getDuration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout:  getDuration("HTTP_SHUTDOWN_TIMEOUT", 20*time.Second),
			AllowedOrigins:   getList("HTTP_ALLOWED_ORIGINS", []string{"*"}),
			OpenAPIBasicUser: getString("OPENAPI_BASIC_USER", ""),
			OpenAPIBasicPass: getString("OPENAPI_BASIC_PASS", ""),
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
			JWTSecret:       getString("AUTH_JWT_SECRET", "dev-secret-change-me"),
			Issuer:          getString("AUTH_ISSUER", "chat-backend"),
			AccessTTL:       getDuration("AUTH_ACCESS_TTL", 15*time.Minute),
			RefreshTTL:      getDuration("AUTH_REFRESH_TTL", 720*time.Hour),
			BcryptCost:      getInt("AUTH_BCRYPT_COST", 12),
			VerificationTTL: getDuration("AUTH_VERIFICATION_TTL", 24*time.Hour),
			ResetTTL:        getDuration("AUTH_PASSWORD_RESET_TTL", time.Hour),
			InviteTTL:       getDuration("AUTH_INVITE_TTL", 72*time.Hour),
		},
		Platform: PlatformConfig{
			APIKeys: parsePlatformKeys(getList("PLATFORM_API_KEYS", nil)),
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
			RatePerMinute:  getInt("PROVIDERHUB_RATE_PER_MINUTE", 60),
			GatewayAPIHost: getString("ISP_GATEWAY_API_HOST", ""),
			GatewayAPIKey:  getString("ISP_GATEWAY_API_KEY", ""),
		},
		MCP: MCPConfig{
			ConsultasURL: getString("SMSNET_MCP_CONSULTAS_URL", ""),
			OperacoesURL: getString("SMSNET_MCP_OPERACOES_URL", ""),
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
		Email: EmailConfig{
			Host:     getString("SMTP_HOST", ""),
			Port:     getInt("SMTP_PORT", 587),
			Username: getString("SMTP_USERNAME", ""),
			Password: getString("SMTP_PASSWORD", ""),
			From:     getString("SMTP_FROM", getString("NOTIFICATIONS_EMAIL_FROM", "no-reply@example.com")),
			TLSMode:  getString("SMTP_TLS_MODE", "starttls"),
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
		Reports: ReportsConfig{
			StorageDir:      getString("REPORTS_STORAGE_DIR", "/tmp/chat-reports"),
			SigningSecret:   getString("REPORTS_SIGNING_SECRET", getString("AUTH_JWT_SECRET", "dev-secret-change-me")),
			DownloadBaseURL: getString("REPORTS_DOWNLOAD_BASE_URL", "http://localhost:8080"),
			DownloadTTL:     getDuration("REPORTS_DOWNLOAD_TTL", 24*time.Hour),
		},
		Attachments: AttachmentsConfig{
			// STORAGE_PROVIDER is the canonical knob (local|s3); ATTACHMENTS_PROVIDER
			// is accepted as a fallback for older deployments.
			Provider:            getString("STORAGE_PROVIDER", getString("ATTACHMENTS_PROVIDER", "local")),
			MaxSizeBytes:        int64(getInt("ATTACHMENTS_MAX_SIZE_BYTES", 25<<20)),
			AvatarMaxSizeBytes:  int64(getInt("ATTACHMENTS_AVATAR_MAX_SIZE_BYTES", 5<<20)),
			AllowedContentTypes: getList("ATTACHMENTS_ALLOWED_CONTENT_TYPES", nil),
			UploadTTL:           getDuration("ATTACHMENTS_UPLOAD_TTL", 15*time.Minute),
			DownloadTTL:         getDuration("ATTACHMENTS_DOWNLOAD_TTL", 5*time.Minute),
			AvatarURLTTL:        getDuration("ATTACHMENTS_AVATAR_URL_TTL", 15*time.Minute),
			SigningSecret:       getString("ATTACHMENTS_SIGNING_SECRET", getString("AUTH_JWT_SECRET", "dev-secret-change-me")),
			LocalDir:            getString("ATTACHMENTS_LOCAL_DIR", "/tmp/chat-attachments"),
			BaseURL:             getString("ATTACHMENTS_BASE_URL", "http://localhost:8080"),
			S3: AttachmentsS3Config{
				Endpoint:       getString("S3_ENDPOINT", getString("ATTACHMENTS_S3_ENDPOINT", "")),
				Region:         getString("S3_REGION", getString("ATTACHMENTS_S3_REGION", "us-east-1")),
				Bucket:         getString("S3_BUCKET", getString("ATTACHMENTS_S3_BUCKET", "")),
				AccessKey:      getString("AWS_ACCESS_KEY_ID", getString("ATTACHMENTS_S3_ACCESS_KEY", "")),
				SecretKey:      getString("AWS_SECRET_ACCESS_KEY", getString("ATTACHMENTS_S3_SECRET_KEY", "")),
				ForcePathStyle: getBool("S3_FORCE_PATH_STYLE", false),
				PresignExpiry:  getDuration("S3_PRESIGN_EXPIRY", 5*time.Minute),
				// Self-heal the bucket CORS on boot so browser-direct uploads work
				// without a manual aws s3api step. Origins reuse HTTP_ALLOWED_ORIGINS.
				EnsureCORS:         getBool("S3_ENSURE_CORS", true),
				CORSAllowedOrigins: getList("S3_CORS_ALLOWED_ORIGINS", getList("HTTP_ALLOWED_ORIGINS", []string{"*"})),
			},
		},
		Seed: SeedConfig{
			TenantName:    getString("SEED_TENANT_NAME", "Default Tenant"),
			OwnerEmail:    getString("SEED_OWNER_EMAIL", "owner@example.com"),
			OwnerName:     getString("SEED_OWNER_NAME", "Owner"),
			OwnerPassword: getString("SEED_OWNER_PASSWORD", "change-me-now"),
			DemoData:      getBool("SEED_DEMO_DATA", false),
			DemoReset:     getBool("SEED_DEMO_RESET", false),
			DemoPassword:  getString("SEED_DEMO_PASSWORD", "demo1234"),
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

// parsePlatformKeys parses entries of the form "key_id:sha256hex" into a map of
// key id -> secret hash. Malformed entries (missing ":" or empty parts) are
// skipped. The platform secret is never stored — only its hash.
func parsePlatformKeys(entries []string) map[string]string {
	if len(entries) == 0 {
		return nil
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		id, hash, ok := strings.Cut(e, ":")
		id = strings.TrimSpace(id)
		hash = strings.ToLower(strings.TrimSpace(hash))
		if !ok || id == "" || hash == "" {
			continue
		}
		out[id] = hash
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func getList(key string, def []string) []string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		// Defensive: strip a leaked inline comment. godotenv only strips an inline
		// "# comment" when the value before it is non-empty, so a line like
		// `KEY=    # note` yields the comment AS the value. These keys are
		// comma-lists where '#' is never a valid token, so cut at the first '#'.
		if i := strings.IndexByte(v, '#'); i >= 0 {
			v = v[:i]
		}
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
