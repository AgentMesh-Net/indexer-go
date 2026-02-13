package config

import (
	"encoding/json"
	"os"
	"strconv"
)

// ChainConfig describes a supported chain.
type ChainConfig struct {
	ChainID            int    `json:"chain_id"`
	SettlementContract string `json:"settlement_contract"`
	MinConfirmations   int    `json:"min_confirmations"`
}

// Config holds application configuration from environment variables.
type Config struct {
	DBDSN        string
	HTTPAddr     string
	MaxBodyBytes int64

	// Indexer identity (Phase 5)
	IndexerName    string
	IndexerBaseURL string
	IndexerOwner   string
	IndexerContact string
	FeeBPS         int
	Version        string
	Commit         string

	// Ed25519 signing key (32-byte hex)
	SigningKeyHex string

	// Supported chains (JSON array)
	SupportedChains []ChainConfig
}

// Load reads configuration from environment variables with defaults.
func Load() Config {
	c := Config{
		DBDSN:        envOr("AMN_DB_DSN", envOr("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/indexer?sslmode=disable")),
		HTTPAddr:     envOr("AMN_HTTP_ADDR", ":8080"),
		MaxBodyBytes: 2 * 1024 * 1024, // 2MB default

		IndexerName:    envOr("INDEXER_NAME", "ainerwise-official-sepolia"),
		IndexerBaseURL: envOr("INDEXER_BASE_URL", "https://indexer.ainerwise.com"),
		IndexerOwner:   envOr("INDEXER_OWNER", "ainerwise"),
		IndexerContact: envOr("INDEXER_CONTACT", "ops@ainerwise.com"),
		FeeBPS:         envInt("INDEXER_FEE_BPS", 20),
		Version:        envOr("INDEXER_VERSION", "1.0.0"),
		Commit:         envOr("INDEXER_COMMIT", ""),

		SigningKeyHex: envOr("INDEXER_SIGNING_KEY", ""),

		SupportedChains: parseChains(envOr("SUPPORTED_CHAINS_JSON",
			`[{"chain_id":11155111,"settlement_contract":"0xf2223eA479736FA2c70fa0BB1430346D937C7C3C","min_confirmations":2}]`)),
	}
	return c
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func parseChains(raw string) []ChainConfig {
	var chains []ChainConfig
	if err := json.Unmarshal([]byte(raw), &chains); err != nil {
		return []ChainConfig{
			{ChainID: 11155111, SettlementContract: "0xf2223eA479736FA2c70fa0BB1430346D937C7C3C", MinConfirmations: 2},
		}
	}
	return chains
}
