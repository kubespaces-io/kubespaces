package server

import (
	"fmt"
	"regexp"
)

// MaxTenantNameLength leaves room for the kubespaces-tenant- namespace prefix.
const MaxTenantNameLength = 40

var dns1123Label = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// ValidateTenantName enforces DNS-1123 label rules with a 40 char cap.
func ValidateTenantName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len(name) > MaxTenantNameLength {
		return fmt.Errorf("name must be at most %d characters", MaxTenantNameLength)
	}
	if !dns1123Label.MatchString(name) {
		return fmt.Errorf("name must be a DNS-1123 label (lowercase alphanumeric and '-', starting and ending with an alphanumeric)")
	}
	return nil
}
