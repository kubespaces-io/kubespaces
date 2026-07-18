package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testToken = "test-token"

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	token := func(context.Context) (string, error) { return testToken, nil }
	return New(srv.URL, srv.Client(), token)
}

func TestMe(t *testing.T) {
	// Arrange
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/me" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
			t.Errorf("Authorization = %q, want bearer token", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"subject": "sub-1",
			"email":   "user@example.com",
			"roles":   []string{"kubespaces-member"},
		})
	})

	// Act
	me, err := c.Me(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Me() error = %v", err)
	}
	if me.Subject != "sub-1" || me.Email != "user@example.com" || len(me.Roles) != 1 {
		t.Errorf("Me() = %+v, want sub-1/user@example.com/1 role", me)
	}
}

func TestListTenants(t *testing.T) {
	// Arrange
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/tenants" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode([]map[string]any{
			{"name": "alpha", "phase": "Ready", "createdAt": "2026-07-18T10:00:00Z"},
			{"name": "beta", "phase": "Provisioning"},
		})
	})

	// Act
	tenants, err := c.ListTenants(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("ListTenants() error = %v", err)
	}
	if len(tenants) != 2 || tenants[0].Name != "alpha" || tenants[1].Phase != "Provisioning" {
		t.Errorf("ListTenants() = %+v", tenants)
	}
	want := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	if !tenants[0].CreatedAt.Equal(want) {
		t.Errorf("CreatedAt = %v, want %v", tenants[0].CreatedAt, want)
	}
}

func TestCreateTenant(t *testing.T) {
	// Arrange
	var gotBody map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/tenants" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"name": "alpha", "phase": "Pending"})
	})
	req := &CreateTenantRequest{
		Name:        "alpha",
		DisplayName: "Alpha",
		Resources:   &Resources{CPU: "4", Memory: "8Gi", Storage: "20Gi"},
	}

	// Act
	tenant, err := c.CreateTenant(context.Background(), req)

	// Assert
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if tenant.Name != "alpha" || tenant.Phase != "Pending" {
		t.Errorf("CreateTenant() = %+v", tenant)
	}
	if gotBody["name"] != "alpha" || gotBody["displayName"] != "Alpha" {
		t.Errorf("request body = %v", gotBody)
	}
	res, ok := gotBody["resources"].(map[string]any)
	if !ok || res["cpu"] != "4" || res["memory"] != "8Gi" || res["storage"] != "20Gi" {
		t.Errorf("request resources = %v", gotBody["resources"])
	}
}

func TestGetTenant(t *testing.T) {
	// Arrange
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/tenants/alpha" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"name": "alpha", "phase": "Ready"})
	})

	// Act
	tenant, err := c.GetTenant(context.Background(), "alpha")

	// Assert
	if err != nil {
		t.Fatalf("GetTenant() error = %v", err)
	}
	if tenant.Name != "alpha" || tenant.Phase != "Ready" {
		t.Errorf("GetTenant() = %+v", tenant)
	}
}

func TestDeleteTenant(t *testing.T) {
	// Arrange
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/tenants/alpha" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
	})

	// Act
	err := c.DeleteTenant(context.Background(), "alpha")

	// Assert
	if err != nil {
		t.Fatalf("DeleteTenant() error = %v", err)
	}
}

func TestKubeconfig(t *testing.T) {
	// Arrange
	const kubeconfigYAML = "apiVersion: v1\nkind: Config\n"
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tenants/alpha/kubeconfig" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/yaml")
		w.Write([]byte(kubeconfigYAML))
	})

	// Act
	data, err := c.Kubeconfig(context.Background(), "alpha")

	// Assert
	if err != nil {
		t.Fatalf("Kubeconfig() error = %v", err)
	}
	if string(data) != kubeconfigYAML {
		t.Errorf("Kubeconfig() = %q, want %q", data, kubeconfigYAML)
	}
}

func TestErrorEnvelope(t *testing.T) {
	// Arrange
	tests := []struct {
		name        string
		status      int
		body        string
		wantMessage string
	}{
		{"json envelope", http.StatusNotFound, `{"error": "tenant not found"}`, "tenant not found"},
		{"conflict envelope", http.StatusConflict, `{"error": "tenant already exists"}`, "tenant already exists"},
		{"non-json body", http.StatusInternalServerError, "boom", "API returned HTTP 500"},
		{"empty body", http.StatusForbidden, "", "API returned HTTP 403"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			})

			// Act
			_, err := c.GetTenant(context.Background(), "alpha")

			// Assert
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %v, want *APIError", err)
			}
			if apiErr.StatusCode != tt.status {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.status)
			}
			if apiErr.Error() != tt.wantMessage {
				t.Errorf("Error() = %q, want %q", apiErr.Error(), tt.wantMessage)
			}
		})
	}
}

func TestTokenErrorPropagates(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("request should not reach the server when the token source fails")
	}))
	defer srv.Close()
	wantErr := errors.New("not logged in")
	c := New(srv.URL, srv.Client(), func(context.Context) (string, error) { return "", wantErr })

	// Act
	_, err := c.ListTenants(context.Background())

	// Assert
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want %v", err, wantErr)
	}
}
