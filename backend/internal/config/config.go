package config

import (
	"os"
	"strings"
	"time"
)

const (
	DefaultAddr           = "127.0.0.1:8787"
	DefaultAllowedOrigins = "http://localhost:5173"
	DefaultShutdownGrace  = 5 * time.Second
)

// Config is the process configuration resolved from the environment at startup.
type Config struct {
	Addr           string
	AllowedOrigins []string
	ShutdownGrace  time.Duration
}

// Load reads configuration from the environment, falling back to local defaults.
func Load() Config {
	return Config{
		Addr:           env("QUERYLENS_ADDR", DefaultAddr),
		AllowedOrigins: splitList(env("QUERYLENS_ALLOWED_ORIGINS", DefaultAllowedOrigins)),
		ShutdownGrace:  DefaultShutdownGrace,
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
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
