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

	HTTP     HTTPConfig
	Mongo    MongoConfig
	Redis    RedisConfig
	Asynq    AsynqConfig
	Otel     OtelConfig
	Auth     AuthConfig
	Realtime RealtimeConfig

	// Seed identifies the bootstrap tenant/owner created on first run.
	Seed SeedConfig
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
