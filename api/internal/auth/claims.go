package auth

import (
	"encoding/json"
	"fmt"
	"slices"
)

// Role names carried in Keycloak realm_access.roles.
const (
	RoleAdmin  = "kubespaces-admin"
	RoleMember = "kubespaces-member"
)

// Claims is the identity the API cares about, extracted from a verified token.
type Claims struct {
	Subject string   `json:"subject"`
	Email   string   `json:"email"`
	Roles   []string `json:"roles"`
}

// IsAdmin reports whether the caller holds the admin role.
func (c *Claims) IsAdmin() bool {
	return slices.Contains(c.Roles, RoleAdmin)
}

// Identity is the owner identifier for tenants: email, falling back to subject.
func (c *Claims) Identity() string {
	if c.Email != "" {
		return c.Email
	}
	return c.Subject
}

// rawClaims mirrors the token payload fields we extract.
type rawClaims struct {
	Subject     string `json:"sub"`
	Email       string `json:"email"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// ParseClaims extracts Claims from a raw JSON token payload.
func ParseClaims(payload []byte) (*Claims, error) {
	var raw rawClaims
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("parse token claims: %w", err)
	}
	if raw.Subject == "" {
		return nil, fmt.Errorf("token has no subject")
	}
	return &Claims{
		Subject: raw.Subject,
		Email:   raw.Email,
		Roles:   raw.RealmAccess.Roles,
	}, nil
}
