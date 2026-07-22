package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Endpoints holds the OIDC endpoints kubespaces needs.
type Endpoints struct {
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
}

// Discover fetches {issuer}/.well-known/openid-configuration and returns the
// device authorization and token endpoints.
func Discover(ctx context.Context, httpClient *http.Client, issuer string) (*Endpoints, error) {
	url := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building discovery request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching OIDC discovery document: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery at %s returned %s", url, resp.Status)
	}
	eps := &Endpoints{}
	if err := json.NewDecoder(resp.Body).Decode(eps); err != nil {
		return nil, fmt.Errorf("parsing OIDC discovery document: %w", err)
	}
	if eps.TokenEndpoint == "" {
		return nil, fmt.Errorf("OIDC discovery document from %s has no token_endpoint", issuer)
	}
	if eps.DeviceAuthorizationEndpoint == "" {
		return nil, fmt.Errorf("issuer %s does not advertise a device_authorization_endpoint (device flow disabled?)", issuer)
	}
	return eps, nil
}
