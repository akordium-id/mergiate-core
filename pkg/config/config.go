package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// insecureJWTSecretFallback is the known insecure default. Production must override it.
const insecureJWTSecretFallback = "mergiate-insecure-secret-key-change-in-production"

type Config struct {
	AppEnv             string
	AppPort            string
	AppName            string
	DatabaseURL        string
	DBMaxConns         int32
	DBMinConns         int32
	DBMaxConnIdle      time.Duration
	DBMaxConnLife      time.Duration
	DBPgBouncer        bool     // DB_PGBOUNCER (default: false). When true, disables prepared statements for transaction pooling.
	JWTSecret          string
	JWTExpiry          time.Duration
	CORSAllowedOrigins []string // from CORS_ALLOWED_ORIGINS (comma-separated)
	StoragePath        string   // from STORAGE_PATH

	// HTTP Transport
	HTTP2Enabled bool // HTTP2_ENABLED (default: true). Set false for HTTP/1.1 only.

	// Rate limiting (RATE_LIMIT_*)
	RateLimitEnabled    bool // RATE_LIMIT_ENABLED (default: false)
	RateLimitPerMinute  int  // RATE_LIMIT_PER_MINUTE (default: 300)
	RateLimitBurst      int  // RATE_LIMIT_BURST — unused by in-memory impl, reserved for Redis token bucket

	// gRPC (GRPC_*)
	GRPCEnabled bool   // GRPC_ENABLED (default: true)
	GRPCPort    string // GRPC_PORT (default: "50051")
}

// IsProduction returns true if AppEnv is a production-like environment.
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production" || c.AppEnv == "prod"
}

// ValidateJWTSecret returns an error if the JWT secret is insecure in a production environment.
func (c *Config) ValidateJWTSecret() error {
	if c.JWTSecret == "" || c.JWTSecret == insecureJWTSecretFallback {
		if c.IsProduction() {
			return errors.New("JWT_SECRET must be set to a strong secret in production (current value is empty or the insecure default)")
		}
		// Non-production: warn but continue
		return nil
	}
	return nil
}

// IsInsecureJWTSecret returns true when the JWT secret is empty or the known insecure default.
func IsInsecureJWTSecret(secret string) bool {
	return secret == "" || secret == insecureJWTSecretFallback
}

func Load() (*Config, error) {
	_ = godotenv.Load() // ignore error if .env not present (e.g. in docker or production)

	appEnv := getEnv("APP_ENV", "development")
	appPort := getEnv("APP_PORT", "8080")
	appName := getEnv("APP_NAME", "mergiate-core")

	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5434")
	dbUser := getEnv("DB_USER", "mergiate")
	dbPass := getEnv("DB_PASSWORD", "mergiate_password")
	dbName := getEnv("DB_NAME", "mergiate_core")
	dbSSL := getEnv("DB_SSLMODE", "disable")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			dbUser, dbPass, dbHost, dbPort, dbName, dbSSL,
		)
	}

	maxConns := getEnvAsInt32("DB_MAX_CONNS", 15)
	minConns := getEnvAsInt32("DB_MIN_CONNS", 3)
	maxConnIdle := getEnvAsDuration("DB_MAX_CONN_IDLE", 15*time.Minute)
	maxConnLife := getEnvAsDuration("DB_MAX_CONN_LIFE", 1*time.Hour)
	dbPgBouncer := getEnv("DB_PGBOUNCER", "false") == "true"

	jwtSecret := getEnv("JWT_SECRET", insecureJWTSecretFallback)
	jwtExpiryStr := getEnv("JWT_EXPIRY", "24h")
	jwtExpiry, err := time.ParseDuration(jwtExpiryStr)
	if err != nil {
		jwtExpiry = 24 * time.Hour
	}

	// CORS_ALLOWED_ORIGINS: comma-separated list; empty = no cross-origin.
	// Dev default: allow localhost:3000 for Nuxt dev server.
	corsRaw := getEnv("CORS_ALLOWED_ORIGINS", "")
	var corsOrigins []string
	if corsRaw != "" {
		for o := range strings.SplitSeq(corsRaw, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				corsOrigins = append(corsOrigins, trimmed)
			}
		}
	}

	storagePath := getEnv("STORAGE_PATH", "./storage/uploads")

	http2Enabled := getEnv("HTTP2_ENABLED", "true") == "true"

	rateLimitEnabled := getEnv("RATE_LIMIT_ENABLED", "false") == "true"
	rateLimitPerMinute := int(getEnvAsInt32("RATE_LIMIT_PER_MINUTE", 300))
	rateLimitBurst := int(getEnvAsInt32("RATE_LIMIT_BURST", 50))

	grpcEnabled := getEnv("GRPC_ENABLED", "true") == "true"
	grpcPort := getEnv("GRPC_PORT", "50051")

	cfg := &Config{
		AppEnv:             appEnv,
		AppPort:            appPort,
		AppName:            appName,
		DatabaseURL:        dbURL,
		DBMaxConns:         maxConns,
		DBMinConns:         minConns,
		DBMaxConnIdle:      maxConnIdle,
		DBMaxConnLife:      maxConnLife,
		DBPgBouncer:        dbPgBouncer,
		JWTSecret:          jwtSecret,
		JWTExpiry:          jwtExpiry,
		CORSAllowedOrigins: corsOrigins,
		StoragePath:        storagePath,
		HTTP2Enabled:       http2Enabled,
		RateLimitEnabled:   rateLimitEnabled,
		RateLimitPerMinute: rateLimitPerMinute,
		RateLimitBurst:     rateLimitBurst,
		GRPCEnabled:        grpcEnabled,
		GRPCPort:           grpcPort,
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvAsInt32(key string, fallback int32) int32 {
	valStr := os.Getenv(key)
	if valStr == "" {
		return fallback
	}
	val, err := strconv.ParseInt(valStr, 10, 32)
	if err != nil {
		return fallback
	}
	return int32(val)
}

func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	valStr := os.Getenv(key)
	if valStr == "" {
		return fallback
	}
	d, err := time.ParseDuration(valStr)
	if err != nil {
		return fallback
	}
	return d
}
