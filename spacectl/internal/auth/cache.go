// Package auth implements the OIDC device authorization grant and the
// on-disk token cache used by spacectl.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kubespaces-io/kubespaces/spacectl/internal/config"
)

const (
	credentialsFileName = "credentials.json"
	credentialsPerm     = 0o600
	credentialsDirPerm  = 0o700
)

// Credentials is the cached token set stored at
// ~/.config/spacectl/credentials.json (mode 0600).
type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry"`
}

// CredentialsPath returns the token cache location.
func CredentialsPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, credentialsFileName), nil
}

// LoadCredentials reads the token cache. A missing file returns os.ErrNotExist.
func LoadCredentials(path string) (*Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	creds := &Credentials{}
	if err := json.Unmarshal(data, creds); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return creds, nil
}

// SaveCredentials writes the token cache with 0600 permissions.
func SaveCredentials(path string, creds *Credentials) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding credentials: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), credentialsDirPerm); err != nil {
		return fmt.Errorf("creating credentials directory: %w", err)
	}
	if err := os.WriteFile(path, data, credentialsPerm); err != nil {
		return fmt.Errorf("writing credentials: %w", err)
	}
	return nil
}

// DeleteCredentials removes the token cache; a missing file is not an error.
func DeleteCredentials(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing credentials: %w", err)
	}
	return nil
}
