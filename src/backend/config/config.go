package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all configuration values required by the backend service.
// Values are populated from environment variables, with sensible defaults
// and validation to guarantee the application can start safely.
type Config struct {
	// Server settings
	ServerPort string `env:"SERVER_PORT" default:"8080"` // Port on which the HTTP server listens

	// PostgreSQL connection settings
	DBHost     string `env:"POSTGRES_HOST" default:"localhost"`
	DBPort     string `env:"POSTGRES_PORT" default:"5432"`
	DBUser     string `env:"POSTGRES_USER" required:"true"`
	DBPassword string `env:"POSTGRES_PASSWORD" required:"true"`
	DBName     string `env:"POSTGRES_DB" required:"true"`
	DBSSLMode  string `env:"POSTGRES_SSLMODE" default:"disable"` // disable, require, verify-ca, verify-full

	// AI service (anomaly detector) endpoint
	AIServiceURL string `env:"AI_SERVICE_URL" default:"http://localhost:8000"`

	// JWT authentication
	JWTSecret string `env:"JWT_SECRET" required:"true"`

	// Logging
	LogLevel string `env:"LOG_LEVEL" default:"info"` // debug, info, warn, error

	// Runtime environment (development, production, test)
	Environment string `env:"ENVIRONMENT" default:"development"`
}

// Load reads environment variables, applies defaults, validates the values,
// and returns a ready‑to‑use Config instance.
// It aborts with a descriptive error if any required variable is missing
// or if validation fails.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to process environment variables: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// validate runs a series of checks on the Config fields.
// It ensures ports are numeric and within range, URLs are well‑formed,
// and enumerated values are within the allowed set.
func (c *Config) validate() error {
	if err := validatePort(c.ServerPort, "SERVER_PORT"); err != nil {
		return err
	}
	if err := validatePort(c.DBPort, "POSTGRES_PORT"); err != nil {
		return err
	}
	if err := validateLogLevel(c.LogLevel); err != nil {
		return err
	}
	if err := validateEnv(c.Environment); err != nil {
		return err
	}
	if err := validateURL(c.AIServiceURL, "AI_SERVICE_URL"); err != nil {
		return err
	}
	if strings.TrimSpace(c.DBSSLMode) == "" {
		return fmt.Errorf("POSTGRES_SSLMODE cannot be empty")
	}
	// JWT secret must be at least 32 characters for HS256 security
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters long")
	}
	return nil
}

// validatePort checks that a string represents a valid TCP port (1‑65535).
func validatePort(portStr, varName string) error {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("%s must be a numeric port, got %q", varName, portStr)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535, got %d", varName, port)
	}
	return nil
}

// validateLogLevel ensures the log level is one of the supported values.
func validateLogLevel(level string) error {
	allowed := []string{"debug", "info", "warn", "error"}
	for _, a := range allowed {
		if strings.EqualFold(level, a) {
			return nil
		}
	}
	return fmt.Errorf("LOG_LEVEL must be one of %v, got %q", allowed, level)
}

// validateEnv checks that the environment flag is a known value.
func validateEnv(env string) error {
	allowed := []string{"development", "production", "test"}
	for _, a := range allowed {
		if strings.EqualFold(env, a) {
			return nil
		}
	}
	return fmt.Errorf("ENVIRONMENT must be one of %v, got %q", allowed, env)
}

// validateURL parses a string as a URL and ensures it has a scheme and host.
func validateURL(raw, varName string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s is not a valid URL: %w", varName, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s must include scheme and host (e.g., http://host)", varName)
	}
	return nil
}

// DSN builds a PostgreSQL Data Source Name suitable for sql.Open.
// Example: "host=localhost port=5432 user=foo password=bar dbname=baz sslmode=disable"
func (c *Config) DSN() string {
	parts := []string{
		fmt.Sprintf("host=%s", c.DBHost),
		fmt.Sprintf("port=%s", c.DBPort),
		fmt.Sprintf("user=%s", c.DBUser),
		fmt.Sprintf("password=%s", c.DBPassword),
		fmt.Sprintf("dbname=%s", c.DBName),
		fmt.Sprintf("sslmode=%s", c.DBSSLMode),
	}
	return strings.Join(parts, " ")
}

// Helper to fetch a configuration value directly from the environment
// without loading the full Config. Useful for tooling that only needs a
// single variable (e.g., CI scripts). Returns empty string if not set.
func GetEnv(key string) string {
	return os.Getenv(key)
}