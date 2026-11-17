package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultAddr           = "127.0.0.1:8787"
	DefaultAllowedOrigins = "http://localhost:5173"
	DefaultShutdownGrace  = 5 * time.Second

	DefaultQueryMaxSQLLength = 10_000
	DefaultQueryTimeout      = 5 * time.Second
	DefaultQueryMaxRows      = 1_000
	DefaultQuerySampleLimit  = 5
)

// Config is the process configuration resolved from the environment at startup.
type Config struct {
	Addr           string
	AllowedOrigins []string
	ShutdownGrace  time.Duration

	// QueryMaxSQLLength bounds the SQL text accepted by the query endpoint.
	QueryMaxSQLLength int
	// QueryTimeout bounds wall-clock execution for one query.
	QueryTimeout time.Duration
	// QueryMaxRows bounds the number of result rows returned to the browser.
	QueryMaxRows int
	// QuerySampleLimit bounds the rows retained at each execution-graph sample.
	QuerySampleLimit int
}

// Load reads configuration from the environment, falling back to local defaults.
// Invalid numeric or duration values fall back to their defaults rather than
// failing startup; the defaults are safe for local use.
func Load() Config {
	return Config{
		Addr:              env("QUERYLENS_ADDR", DefaultAddr),
		AllowedOrigins:    splitList(env("QUERYLENS_ALLOWED_ORIGINS", DefaultAllowedOrigins)),
		ShutdownGrace:     DefaultShutdownGrace,
		QueryMaxSQLLength: envInt("QUERYLENS_QUERY_MAX_SQL_LENGTH", DefaultQueryMaxSQLLength),
		QueryTimeout:      envDuration("QUERYLENS_QUERY_TIMEOUT", DefaultQueryTimeout),
		QueryMaxRows:      envInt("QUERYLENS_QUERY_MAX_ROWS", DefaultQueryMaxRows),
		QuerySampleLimit:  envInt("QUERYLENS_QUERY_SAMPLE_LIMIT", DefaultQuerySampleLimit),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}

func splitList(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
