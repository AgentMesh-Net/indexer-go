package config

import "os"

// Config holds application configuration from environment variables.
type Config struct {
	DBDSN        string
	HTTPAddr     string
	MaxBodyBytes int64
}

// Load reads configuration from environment variables with defaults.
func Load() Config {
	c := Config{
		DBDSN:        envOr("AMN_DB_DSN", envOr("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/indexer?sslmode=disable")),
		HTTPAddr:     envOr("AMN_HTTP_ADDR", ":8080"),
		MaxBodyBytes: 2 * 1024 * 1024, // 2MB default
	}
	return c
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
