// Package client is a thin, hand-written HTTP client for the KubeSpaces
// /api/v1 backend (see docs/contracts.md).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	basePath        = "/api/v1"
	maxErrorBody    = 1 << 20
	defaultTimeout  = 30 * time.Second
	contentTypeJSON = "application/json"
)

// TokenFunc supplies the bearer token for each request.
type TokenFunc func(ctx context.Context) (string, error)

// Client talks to the KubeSpaces backend API.
type Client struct {
	baseURL string
	http    *http.Client
	token   TokenFunc
}

// New creates a Client for the given server (portal URL, no trailing slash
// required). token may be nil for unauthenticated use in tests.
func New(server string, httpClient *http.Client, token TokenFunc) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		baseURL: strings.TrimSuffix(server, "/") + basePath,
		http:    httpClient,
		token:   token,
	}
}

// Me is the response of GET /api/v1/me.
type Me struct {
	Subject string   `json:"subject"`
	Email   string   `json:"email"`
	Roles   []string `json:"roles"`
}

// Resources is the tenant resource envelope.
type Resources struct {
	CPU     string `json:"cpu,omitempty"`
	Memory  string `json:"memory,omitempty"`
	Storage string `json:"storage,omitempty"`
}

// Tenant mirrors the Tenant JSON in docs/contracts.md.
type Tenant struct {
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Owner       string    `json:"owner"`
	Phase       string    `json:"phase"`
	Message     string    `json:"message,omitempty"`
	Resources   Resources `json:"resources"`
	CreatedAt   time.Time `json:"createdAt"`
}

// CreateTenantRequest is the body of POST /api/v1/tenants.
type CreateTenantRequest struct {
	Name        string     `json:"name"`
	DisplayName string     `json:"displayName,omitempty"`
	Resources   *Resources `json:"resources,omitempty"`
}

// APIError is the backend's {"error": "..."} envelope plus the HTTP status.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("API returned HTTP %d", e.StatusCode)
}

// Me fetches the authenticated identity.
func (c *Client) Me(ctx context.Context) (*Me, error) {
	me := &Me{}
	if err := c.doJSON(ctx, http.MethodGet, "/me", nil, me); err != nil {
		return nil, err
	}
	return me, nil
}

// ListTenants fetches all tenants visible to the caller.
func (c *Client) ListTenants(ctx context.Context) ([]Tenant, error) {
	var tenants []Tenant
	if err := c.doJSON(ctx, http.MethodGet, "/tenants", nil, &tenants); err != nil {
		return nil, err
	}
	return tenants, nil
}

// CreateTenant creates a tenant and returns the created object.
func (c *Client) CreateTenant(ctx context.Context, req *CreateTenantRequest) (*Tenant, error) {
	tenant := &Tenant{}
	if err := c.doJSON(ctx, http.MethodPost, "/tenants", req, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

// GetTenant fetches one tenant by name.
func (c *Client) GetTenant(ctx context.Context, name string) (*Tenant, error) {
	tenant := &Tenant{}
	if err := c.doJSON(ctx, http.MethodGet, "/tenants/"+url.PathEscape(name), nil, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

// DeleteTenant requests deletion of a tenant (API answers 202).
func (c *Client) DeleteTenant(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodDelete, "/tenants/"+url.PathEscape(name), nil, nil)
}

// Kubeconfig fetches the raw kubeconfig YAML for a tenant.
func (c *Client) Kubeconfig(ctx context.Context, name string) ([]byte, error) {
	resp, err := c.do(ctx, http.MethodGet, "/tenants/"+url.PathEscape(name)+"/kubeconfig", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading kubeconfig response: %w", err)
	}
	return data, nil
}

// doJSON performs a request with an optional JSON body and decodes a JSON
// response into out (skipped when out is nil or the body is empty).
func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding API response: %w", err)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", contentTypeJSON)
	}
	req.Header.Set("Accept", "*/*")
	if c.token != nil {
		tok, err := c.token(ctx)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling API: %w", err)
	}
	return resp, nil
}

// checkStatus turns non-2xx responses into *APIError, surfacing the
// backend's {"error": "..."} message when present.
func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	var envelope struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error != "" {
		return &APIError{StatusCode: resp.StatusCode, Message: envelope.Error}
	}
	return &APIError{StatusCode: resp.StatusCode}
}
