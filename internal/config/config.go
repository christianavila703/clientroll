package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration.
// The master encryption key is the ONLY secret loaded from env.
// All other credentials are stored encrypted in the database.
type Config struct {
	Port                int
	DatabaseURL         string
	RedisURL            string
	MasterEncryptionKey string
	CORSAllowedOrigins  []string
	Environment         string // "development", "staging", "production"
	JWTAccessDuration   int    // minutes
	JWTRefreshDuration  int    // days
	MailerSendAPIKey    string
	MailerSendFromEmail string
	AppURL              string // e.g. https://app.clientroll.com

	// GoCardless Connect (partner app) — optional; empty disables GoCardless
	GoCardlessSandboxClientID     string
	GoCardlessSandboxClientSecret string
	GoCardlessLiveClientID        string
	GoCardlessLiveClientSecret    string
	GoCardlessWebhookSecret       string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnvInt("PORT", 8080),
		DatabaseURL:        getEnvRequired("DATABASE_URL"),
		RedisURL:           getEnvRequired("REDIS_URL"),
		MasterEncryptionKey: getEnvRequired("MASTER_ENCRYPTION_KEY"),
		CORSAllowedOrigins: getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		Environment:        getEnv("ENVIRONMENT", "development"),
		JWTAccessDuration:   getEnvInt("JWT_ACCESS_DURATION_MINUTES", 15),
		JWTRefreshDuration:  getEnvInt("JWT_REFRESH_DURATION_DAYS", 7),
		MailerSendAPIKey:    getEnv("MAILERSEND_API_KEY", ""),
		MailerSendFromEmail: getEnv("MAILERSEND_FROM_EMAIL", "clientroll@app.businessnet.com"),
		AppURL:              getEnv("APP_URL", "https://app.clientroll.com"),
		GoCardlessSandboxClientID:     getEnv("GOCARDLESS_SANDBOX_CLIENT_ID", ""),
		GoCardlessSandboxClientSecret: getEnv("GOCARDLESS_SANDBOX_CLIENT_SECRET", ""),
		GoCardlessLiveClientID:        getEnv("GOCARDLESS_LIVE_CLIENT_ID", ""),
		GoCardlessLiveClientSecret:    getEnv("GOCARDLESS_LIVE_CLIENT_SECRET", ""),
		GoCardlessWebhookSecret:       getEnv("GOCARDLESS_WEBHOOK_SECRET", ""),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.RedisURL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}
	if c.MasterEncryptionKey == "" {
		return fmt.Errorf("MASTER_ENCRYPTION_KEY is required")
	}
	if len(c.MasterEncryptionKey) < 32 {
		return fmt.Errorf("MASTER_ENCRYPTION_KEY must be at least 32 characters")
	}
	return nil
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvRequired(key string) string {
	v := os.Getenv(key)
	if v == "" {
		// In development, allow empty for easier startup
		// Production will fail at validation
		return ""
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvSlice(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		return strings.Split(v, ",")
	}
	return fallback
}
