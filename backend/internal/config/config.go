// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr            string
	PublicURL       string // public URL of the API, e.g. https://api.modelrouter.app
	WebURL          string // public URL of the website, e.g. https://modelrouter.app
	DatabaseURL     string
	DBMaxConns      int32
	InternalSecret  string // shared secret between the Next.js server and this service
	JWTSecret       string // HS256 secret used by the web server to sign user tokens
	EncryptionKey   string // used to encrypt provider API keys at rest
	AdminEmails     []string
	OpenRouterKey   string // bootstraps the default upstream provider on first run
	OpenRouterURL   string
	AppName         string
	CatalogRefresh  time.Duration
	ModelSyncEvery  time.Duration
	FlushInterval   time.Duration
	MaxBodyBytes    int64
	UpstreamTimeout time.Duration
	CORSOrigins     []string
	LogLevel        string

	Paddle     PaddleConfig
	SSLCommerz SSLCommerzConfig
}

type PaddleConfig struct {
	APIKey        string
	WebhookSecret string
	Environment   string // sandbox | production
	ProductID     string // optional catalog product used for top-ups
}

type SSLCommerzConfig struct {
	StoreID     string
	StorePasswd string
	Sandbox     bool
}

func Load() (*Config, error) {
	c := &Config{
		Addr:            env("ADDR", ":8080"),
		PublicURL:       strings.TrimRight(env("PUBLIC_URL", "http://localhost:8080"), "/"),
		WebURL:          strings.TrimRight(env("WEB_URL", "http://localhost:3000"), "/"),
		DatabaseURL:     env("DATABASE_URL", "postgres://modelrouter:modelrouter@localhost:5432/modelrouter?sslmode=disable"),
		DBMaxConns:      int32(envInt("DB_MAX_CONNS", 32)),
		InternalSecret:  os.Getenv("INTERNAL_SECRET"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		EncryptionKey:   os.Getenv("ENCRYPTION_KEY"),
		AdminEmails:     splitList(os.Getenv("ADMIN_EMAILS")),
		OpenRouterKey:   os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterURL:   strings.TrimRight(env("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"), "/"),
		AppName:         env("APP_NAME", "ModelRouter"),
		CatalogRefresh:  envDuration("CATALOG_REFRESH", 30*time.Second),
		ModelSyncEvery:  envDuration("MODEL_SYNC_INTERVAL", 6*time.Hour),
		FlushInterval:   envDuration("BILLING_FLUSH_INTERVAL", time.Second),
		MaxBodyBytes:    int64(envInt("MAX_BODY_BYTES", 32<<20)),
		UpstreamTimeout: envDuration("UPSTREAM_TIMEOUT", 10*time.Minute),
		CORSOrigins:     splitList(env("CORS_ORIGINS", "*")),
		LogLevel:        env("LOG_LEVEL", "info"),
		Paddle: PaddleConfig{
			APIKey:        os.Getenv("PADDLE_API_KEY"),
			WebhookSecret: os.Getenv("PADDLE_WEBHOOK_SECRET"),
			Environment:   env("PADDLE_ENV", "sandbox"),
			ProductID:     os.Getenv("PADDLE_PRODUCT_ID"),
		},
		SSLCommerz: SSLCommerzConfig{
			StoreID:     os.Getenv("SSLCOMMERZ_STORE_ID"),
			StorePasswd: os.Getenv("SSLCOMMERZ_STORE_PASSWORD"),
			Sandbox:     env("SSLCOMMERZ_SANDBOX", "true") == "true",
		},
	}
	var errs []error
	if len(c.InternalSecret) < 16 {
		errs = append(errs, errors.New("INTERNAL_SECRET must be at least 16 characters"))
	}
	if len(c.JWTSecret) < 16 {
		errs = append(errs, errors.New("JWT_SECRET must be at least 16 characters"))
	}
	if len(c.EncryptionKey) < 16 {
		errs = append(errs, errors.New("ENCRYPTION_KEY must be at least 16 characters"))
	}
	return c, errors.Join(errs...)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

func envDuration(k string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, strings.ToLower(p))
		}
	}
	return out
}
