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
	JWTSecret          string
	JWTExpiry          time.Duration
	CORSAllowedOrigins []string // from CORS_ALLOWED_ORIGINS (comma-separated)
	StoragePath        string   // from STORAGE_PATH
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

	maxConns := getEnvAsInt32("DB_MAX_CONNS", 25)
	minConns := getEnvAsInt32("DB_MIN_CONNS", 5)

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

	cfg := &Config{
		AppEnv:             appEnv,
		AppPort:            appPort,
		AppName:            appName,
		DatabaseURL:        dbURL,
		DBMaxConns:         maxConns,
		DBMinConns:         minConns,
		DBMaxConnIdle:      15 * time.Minute,
		DBMaxConnLife:      1 * time.Hour,
		JWTSecret:          jwtSecret,
		JWTExpiry:          jwtExpiry,
		CORSAllowedOrigins: corsOrigins,
		StoragePath:        storagePath,
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
