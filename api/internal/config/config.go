// Package config loads API configuration from environment variables.
package config

import (
	"fmt"
	"os"
)

// Config holds all runtime configuration for the API.
type Config struct {
	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string

	OIDCIssuerURL string
	OIDCClientID  string

	ListenAddr string
}

const defaultListenAddr = ":8080"
const defaultDBPort = "5432"

// Load reads configuration from the environment and fails fast on
// missing required variables.
func Load() (*Config, error) {
	cfg := &Config{
		DBHost:        os.Getenv("KUBESPACES_DB_HOST"),
		DBPort:        envOr("KUBESPACES_DB_PORT", defaultDBPort),
		DBName:        os.Getenv("KUBESPACES_DB_NAME"),
		DBUser:        os.Getenv("KUBESPACES_DB_USER"),
		DBPassword:    os.Getenv("KUBESPACES_DB_PASSWORD"),
		OIDCIssuerURL: os.Getenv("KUBESPACES_OIDC_ISSUER_URL"),
		OIDCClientID:  os.Getenv("KUBESPACES_OIDC_CLIENT_ID"),
		ListenAddr:    envOr("KUBESPACES_LISTEN_ADDR", defaultListenAddr),
	}

	required := map[string]string{
		"KUBESPACES_DB_HOST":         cfg.DBHost,
		"KUBESPACES_DB_NAME":         cfg.DBName,
		"KUBESPACES_DB_USER":         cfg.DBUser,
		"KUBESPACES_DB_PASSWORD":     cfg.DBPassword,
		"KUBESPACES_OIDC_ISSUER_URL": cfg.OIDCIssuerURL,
		"KUBESPACES_OIDC_CLIENT_ID":  cfg.OIDCClientID,
	}
	var missing []string
	for name, value := range required {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}
	return cfg, nil
}

// DSN returns a pgx keyword/value connection string.
func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s",
		c.DBHost, c.DBPort, c.DBName, c.DBUser, c.DBPassword)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
