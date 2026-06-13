// Package config loads runtime configuration from environment variables
// into a single typed struct. One place owns "which env vars exist",
// what they default to, and which are required — keeps the rest of the
// code from sprinkling os.Getenv calls across every package (Rule #1:
// reusable, one source of truth).
//
// Names mirror the NestJS backend's .env exactly (JWT_SECRET,
// DATABASE_URL, …) so a single .env file can boot both services.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is the immutable runtime configuration. Built once in main()
// then passed (or its fields passed) into every adapter that needs it.
// Never mutate after construction.
type Config struct {
	// HTTP listen address. Default ":3001" matches the docker-compose
	// port mapping; override via LISTEN_ADDR.
	Listen string

	// Verbosity threshold for slog. Accepted: debug | info | warn | error.
	LogLevel string

	// HS256 signing secret. MUST match the NestJS JWT_SECRET byte-for-byte
	// or every token rejects. Required.
	JWTSecret string

	// HS256 secret for REFRESH tokens. Mirrors the NestJS
	// JWT_REFRESH_SECRET. Distinct from JWTSecret (access tokens).
	JWTRefreshSecret string

	// Postgres DSN. Required from Task 3 onward; permitted empty now so
	// Task 2 tests run without spinning up Postgres.
	DatabaseURL string

	// Redis URL. Required from Task 6 onward.
	RedisURL string

	// AI provider keys — required from Task 6 onward, not yet.
	OpenAIKey    string
	AnthropicKey string

	// OpenAIProviderID lets the operator pin the openai adapter's
	// ai_providers.id to a row that already exists in Postgres,
	// instead of letting the adapter mint a fresh uuid.New() on
	// every restart. With this set the Go service can write
	// usage_logs.ai_provider_id rows that satisfy the existing FK
	// constraint against ai_providers — i.e. analytics queries
	// joining usage_logs ↔ ai_providers see the Go-served rewrites
	// alongside the NestJS-served ones under the same provider row.
	// Empty = mint a fresh UUID (dev / no-FK use case).
	OpenAIProviderID    string
	AnthropicProviderID string
	OllamaProviderID    string

	// OllamaURL points at a local or proxied Ollama server. Empty
	// disables the Ollama adapter; "http://localhost:11434" is the
	// canonical local dev value.
	OllamaURL string

	// AIProviders is an ordered, comma-separated provider priority
	// list — used by the failover chain in composeDeps. Example:
	//   AI_PROVIDERS=openai,anthropic,ollama
	// Unknown entries + entries whose credentials are missing get
	// filtered out at wiring time with a warning. Empty string =
	// "use the memory stub" (dev convenience).
	AIProviders string

	// GoBackendRampPercent is the percentage of users bucketed onto the
	// Go backend, surfaced via /auth/me flags.use_go_backend. Mirrors the
	// Node GO_BACKEND_RAMP_PERCENT env var. Default 0 (no ramp).
	GoBackendRampPercent int

	// App environment label (development | staging | production).
	// Drives a few startup checks + the log output format choice.
	AppEnv string

	// MetricsEnabled gates the /metrics Prometheus endpoint. Default
	// off — exposing it on a public listener leaks timing+cardinality
	// info, so production wires a separate internal listener (TBD)
	// or fronts it with auth.
	MetricsEnabled bool

	// OtelEndpoint is OTEL_EXPORTER_OTLP_ENDPOINT (host:port). Empty
	// disables tracing entirely — global TracerProvider stays noop.
	OtelEndpoint string

	// OtelSampleRatio is the head-based sample rate for traces.
	// 1.0 = all, 0.1 = 10%. Default 1.0 to make dev visible.
	OtelSampleRatio float64
}

// Load reads env vars into a Config and validates required fields.
// Returns a wrapped error listing every missing-required field, so the
// operator can fix all of them in one shot instead of one-error-at-a-time.
func Load() (*Config, error) {
	c := &Config{
		Listen:               envOr("LISTEN_ADDR", ":3001"),
		LogLevel:             envOr("LOG_LEVEL", "info"),
		JWTSecret:            os.Getenv("JWT_SECRET"),
		JWTRefreshSecret:     os.Getenv("JWT_REFRESH_SECRET"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		RedisURL:             os.Getenv("REDIS_URL"),
		OpenAIKey:            os.Getenv("OPENAI_API_KEY"),
		AnthropicKey:         os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIProviderID:     os.Getenv("OPENAI_PROVIDER_ID"),
		AnthropicProviderID:  os.Getenv("ANTHROPIC_PROVIDER_ID"),
		OllamaProviderID:     os.Getenv("OLLAMA_PROVIDER_ID"),
		OllamaURL:            os.Getenv("OLLAMA_URL"),
		AIProviders:          os.Getenv("AI_PROVIDERS"),
		GoBackendRampPercent: envInt("GO_BACKEND_RAMP_PERCENT", 0),
		AppEnv:               envOr("APP_ENV", "development"),

		MetricsEnabled:  envBool("METRICS_ENABLED", false),
		OtelEndpoint:    os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		OtelSampleRatio: envFloat("OTEL_SAMPLE_RATIO", 1.0),
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// validate returns nil when all required-for-the-current-task fields are
// set. As more tasks land, additional fields move from "permitted empty"
// to "required" — gate them here so we never start with half a config.
func (c *Config) validate() error {
	var missing []string
	if strings.TrimSpace(c.JWTSecret) == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if !validLogLevel(c.LogLevel) {
		return fmt.Errorf("config: LOG_LEVEL must be one of debug|info|warn|error, got %q", c.LogLevel)
	}
	if len(missing) > 0 {
		return fmt.Errorf("config: required env vars missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

// IsProduction is the canonical place to gate "behaviour different in
// prod vs dev" — never literal `== "production"` scattered around.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

// envOr returns the env var value or a fallback when unset/empty.
// Single helper rather than repeated `if v := …; v != "" { … } else { … }`.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envBool parses a boolean env var. "1", "true", "yes", "on" (case-
// insensitive) → true; everything else (including unset) → fallback.
// Centralised so feature flags read consistently across the codebase.
func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

// envInt parses an int env var; returns fallback on unset/parse error.
func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// envFloat parses a float64 env var; returns fallback on parse error.
func envFloat(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func validLogLevel(s string) bool {
	switch strings.ToLower(s) {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

// ErrConfigInvalid is the sentinel callers can errors.Is against when
// they want to distinguish config-load failures from runtime errors.
var ErrConfigInvalid = errors.New("config invalid")
