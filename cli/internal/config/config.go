// Package config loads and saves the kubespaces configuration file
// (~/.config/kubespaces/config.yaml).
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"
)

// EnvServer is the environment variable that overrides the configured server.
const EnvServer = "KUBESPACES_CLI_SERVER"

const (
	dirName        = "kubespaces"
	configFileName = "config.yaml"
	dirPerm        = 0o700
	filePerm       = 0o600
)

// Config is the persisted kubespaces configuration.
type Config struct {
	Server   string `json:"server,omitempty"`
	Issuer   string `json:"issuer,omitempty"`
	ClientID string `json:"clientId,omitempty"`
}

// Dir returns the kubespaces configuration directory, honoring
// XDG_CONFIG_HOME and defaulting to ~/.config/kubespaces.
func Dir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, dirName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, ".config", dirName), nil
}

// Path returns the full path of the config file.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// Load reads the config file. A missing file yields an empty Config.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads a config file from an explicit path.
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config file, creating the directory if needed.
func Save(cfg *Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	return SaveTo(path, cfg)
}

// SaveTo writes a config file to an explicit path.
func SaveTo(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.WriteFile(path, data, filePerm); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// ResolveServer applies the precedence flag > KUBESPACES_CLI_SERVER env > config.
func (c *Config) ResolveServer(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv(EnvServer); env != "" {
		return env
	}
	return c.Server
}
